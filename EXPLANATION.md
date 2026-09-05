# Why txi used 130MB to open a 23MB file, and how it now uses 8.8MB

A walkthrough of the change, assuming no familiarity with the code.

---

## 1. The measurement

Opening `test.txt` (23.3MB, 202,000 lines, ~120 characters per line) and pressing `G`
to jump to the end:

| | peak memory (RSS) |
| --- | --- |
| before | **130.2 MB** |
| after | **8.8 MB** |

More interesting than the ratio is where the new number comes from:

| | peak memory |
| --- | --- |
| txi opening a **1-line** file | 5.22 MB |
| txi opening the **23.3MB** file | 7.83 MB |

So the entire marginal cost of a 23MB file is now **2.6MB**. The 5.2MB floor is the Go
runtime and termbox's own screen buffers — it's there whatever you open, and it's the
same floor the old version had.

"RSS" (resident set size) is what Activity Monitor and `ps` show: the memory your
process actually occupies in physical RAM right now.

---

## 2. Why the old version cost 130MB

The old code did this:

```go
var textBuf [][]rune

scanner := bufio.NewScanner(file)
for scanner.Scan() {
    textBuf = append(textBuf, []rune(scanner.Text()))
}
```

Read the whole file, split it into lines, decode every line into `[]rune`, keep all of
it. Simple and obvious — and that's exactly what costs 130MB. Three separate multipliers
are stacked on top of each other.

### Multiplier 1: `rune` is 4 bytes, not 1

A `rune` in Go is an `int32` — a Unicode code point, always 4 bytes wide, whether it
holds `a` or `漢`. Your file is ASCII, one byte per character on disk. Decoding it to
`[]rune` therefore **quadruples it before anything else happens**:

```
23.3 MB on disk  →  93.2 MB as runes
```

This is the single biggest factor, and it's invisible in the source. `[]rune(line)`
looks free. It is a 4× memory multiplier.

### Multiplier 2: the allocator rounds every line up

Go doesn't hand out arbitrary byte counts. It has fixed *size classes* (16, 32, 48, 64,
80, 96, 112, 128, 144, 160, 176, 192, 208, 224, 240, 256, 288, 320, 352, 384, 416, 448,
480, 512, 576, 640, 704, 768, 896, 1024 …) and rounds your request up to the next one.

A 120-character line needs 120 × 4 = 480 bytes. That one happens to land exactly on a
class. A 121-character line needs 484 bytes and gets **576** — 19% wasted. Averaged over
200k lines of varying length, you lose several percent to rounding, permanently.

### Multiplier 3: 24 bytes of bookkeeping per line

`textBuf` is a slice *of slices*. Every element is a slice header — pointer (8 bytes),
length (8), capacity (8) = **24 bytes per line**, before any characters.

```
202,000 lines × 24 bytes = 4.85 MB
```

Just to know where the lines are. That's three times the size of the new design's entire
index.

### Adding it up

```
202,000 lines × ~512 bytes (480 of runes, rounded)   ≈ 103.4 MB
202,000 slice headers × 24 bytes                     ≈   4.9 MB
Go runtime + termbox                                 ≈   5.2 MB
scanner garbage the GC hadn't reclaimed yet          ≈  the rest
                                                     ─────────
                                                       ~130 MB
```

There's a fourth, subtler cost. `scanner.Text()` allocates a fresh `string` for every
line, and `[]rune(...)` then allocates again. So the program churns through ~400,000
allocations during load, producing ~93MB of immediately-dead string garbage. Go's garbage
collector doesn't return that to the OS promptly — it keeps the space for reuse. So the
peak sits above the live-data total.

**The core problem: cost scaled with file size, and every byte was paid up front, before
you'd looked at a single line.** A 30-line terminal can display 30 lines. The other
201,970 were decoded, allocated, and held in RAM so you could look at 0.015% of them.

---

## 3. What replaced it

`buffer.go` — the file is **never loaded**. It stays on disk, the file handle stays open,
and the editor holds three small things:

### a. The line index — `starts []int64`

One number per line: the byte offset where that line begins.

```
line 0      → byte 0
line 1      → byte 121
line 2      → byte 243
...
line 201999 → byte 24439870
```

8 bytes per line. For your file: **1.6MB**, and that is the only thing that grows with
file size. Compare: the old version's *slice headers alone*, holding no text at all, were
3× larger than this.

This index is what makes everything else possible. Want line 150,000? It starts at
`starts[150000]` and ends where line 150,001 starts. That's an array lookup — instant,
regardless of file size, without having read anything in between.

### b. The read window — `win []byte`

A single **64KB** buffer holding the raw bytes around wherever the cursor is, plus which
line range it currently covers (`winFrom`, `winTo`).

When you ask for a line inside that range, you get it straight from the buffer. When you
ask for one outside, the buffer is refilled from disk at the new position — reusing the
same 64KB of memory. It never grows.[^1]

64KB holds roughly 500 lines of your file. Since a terminal shows ~30, you can scroll a
long way before it needs to refill.

[^1]: One exception: a single line longer than 64KB forces the window to grow to fit it,
because a line must be readable whole. It shrinks back afterwards. There's a test for it.

### c. The decoded-line cache

Exactly one line, decoded to `[]rune`. The cursor keeps asking about the line it's on —
"how long is it?", "what character is at column 12?" — so caching that one line makes the
`w`/`b`/`e` word motions fast without holding anything else.

### That's it

```
line index         1.6 MB   ← the only part that grows with the file
read window         64 KB   ← fixed forever
decoded line      ~500 B    ← fixed forever
```

The old design was *O(file size)* in the worst way: 5.6 bytes of RAM per byte of file. The
new one is *O(lines)* at 8 bytes per line, plus a constant — and 8 bytes per line is
~15× cheaper than storing the lines themselves.

---

## 4. What actually happens, step by step

### Opening the file

1. Open it, ask the OS how big it is.
2. **Read it once, start to finish, through a single reusable 1MB buffer**, looking only
   for newline bytes and recording their positions. The 1MB buffer is reused for each
   chunk — 23MB of file, 1MB of memory, and it's freed when the pass ends.
3. Read the last byte to learn whether the file ends with a newline (this determines where
   the final line ends).
4. Done. ~50ms, and the editor holds a 1.6MB index and nothing else.

This pass is why `G` is instant and the status bar can say `202000 lines` truthfully. It's
also the one unavoidable cost: **you cannot know how many lines a file has without looking
at all of it.**

### Pressing `j` (down one line)

`currentRow++`. Then the redraw asks for lines `offsetRow` … `offsetRow+30`. They're
already inside the 64KB window, so nothing touches the disk. Roughly every 500 lines of
scrolling, one refill happens.

### Pressing `G` (jump to the end)

`currentRow = LineCount() - 1` — an array length. The redraw asks for the last 30 lines,
they're outside the window, so one 64KB read happens at that offset. **One disk read to
jump to the end of a 23MB file**, no matter how big it gets.

This is the part your paging design would have struggled with. Without a line index, "go
to the end" means either scanning forward from the start, or reading backwards from the
end while counting newlines to figure out what line number you landed on.

### Typing a character

Edited lines go into an **overlay** — a `map[int][]rune` from line number to the edited
content. Every read checks the overlay first and falls back to disk.

This matters more than it looks. In a design where you cache pages of the file and evict
old ones, an edited line sitting in an evicted page is *silently lost*. The overlay is
separate from the window, so the window can move anywhere without touching your edits.
Only lines you've actually modified take up memory. There's a test that edits a line,
forces the window to the far end of the file, comes back, and checks the edit survived.

---

## 5. The mmap detour — what I tried, and why it lost

The textbook answer here is `mmap`: map the file into your address space, then read it
like a giant `[]byte` while the OS pages it in and out for you. It looked ideal — it's
your "keep only what you're looking at" idea, implemented by the kernel, at 4KB
granularity instead of 30 lines, with no bookkeeping.

I built it. It lost. Here's the trap:

**Building the index requires scanning every byte of the file.** With mmap, "reading a
byte" means *faulting the page containing it into physical memory*. Scanning for newlines
therefore pulls the entire 23MB into RAM as a side effect. Measured:

```
after mmap:               4.5 MB   (nothing read yet — mapping is free)
after scanning for \n:   28.4 MB   (the whole file is now resident)
```

Fine, hand the pages back — that's what `madvise` is for. Except on macOS:

```
MADV_DONTNEED        → returns success, frees nothing   (28.4 MB → 28.5 MB)
MADV_FREE            → returns success, frees nothing
MADV_FREE_REUSABLE   → EPERM (operation not permitted on file mappings)
MADV_SEQUENTIAL      → no effect
```

macOS simply will not let you evict clean file-backed pages on demand. They stay resident
until the system is under memory pressure. So the mmap version's RSS was **28MB after
opening and 37MB after scrolling to the bottom**, and it never came down.

The fix is the thing that made everything else work: **read the file with `pread` into a
small reusable buffer instead of walking the mapping.** The data goes through the OS page
cache either way — but page-cache pages are the *kernel's* memory, not charged to your
process, whereas mapped pages you've touched are charged to you. Same bytes read, same
disk work, 1MB of RSS instead of 23MB.

Once indexing no longer needed the mapping, the mapping had no purpose left. Deleting it
also deleted the platform-specific `syscall.Mmap` code and its build tags. The result is
plain portable Go that cross-builds for Linux and Windows unchanged.

> **The general lesson:** with mmap, *reading is allocating*. On Linux you can often undo
> it with `MADV_DONTNEED`. On macOS you can't. If you need to scan a large file once and
> not keep it, read it in chunks — don't map it.

---

## 6. Why this beats the paging design you proposed

Your instinct — "only keep the pages near the cursor" — was right, and it's exactly what
the 64KB window does. The difference is *what it sits on top of*.

The plan was: fetch 2 pages of 30 lines, fetch one more as you scroll, drop the one that
falls off. The problem is that "page N" is not a thing a file has. Files have *bytes*.
Without an index, to display line 900 you have to read from byte 0 counting newlines
until you've seen 900 of them. Every jump becomes a scan.

That forces the full scan anyway — and once you're scanning, three more things fall out of
it for free:

| | 30-line paging | index + window |
| --- | --- | --- |
| memory | ~3 MB | ~2.6 MB |
| `G` to end | scan or read-backwards-and-count | one array lookup |
| line count in status bar | needs a full scan regardless | exact, free |
| edited line in an evicted page | lost unless pinned | can't happen — overlay is separate |
| granularity | 30 lines | 64KB, and self-adjusting to line length |
| code | page table, eviction policy, pinning | one buffer + two integers |

The paging design does more work to get a worse result. Not because the instinct was
wrong, but because it was applied one layer too high: page *bytes*, not *lines*, and let
an index do the line arithmetic.

---

## 7. What this costs

Honest trade-offs:

- **Opening is a full read.** ~50ms for 23MB. Unavoidable if the line count must be exact
  and `G` must be instant. (A lazy version could index in the background and show
  `202000+` until it finishes — worth it only for gigabyte files.)
- **Scrolling does disk reads.** One 64KB `pread` per ~500 lines. It hits the OS page
  cache, so it's microseconds, and the kernel reads ahead when you scroll steadily.
- **8 bytes per line is now the floor.** A 10GB log with a billion lines would need an 8GB
  index. At that scale you'd index every 100th line and scan between them — a known
  technique, and this design extends to it cleanly.
- **`starts` is currently fixed at open.** Nothing inserts or deletes *lines* yet (there's
  no Enter key handling), so this hasn't bitten. When you add `o`/`O`/`dd`, inserting into
  the middle of `starts` becomes O(lines) per edit. The standard fix is a piece table or a
  gap buffer over the index — a natural next step, not a rewrite.
- **Saving isn't implemented** (it wasn't before either). When you add it, the write path
  walks the index and emits either the file's bytes or the overlay's version for each line.
  The structure makes this straightforward — but write to a temp file and rename, since
  you're reading the file you're writing.

---

## 8. How to check it yourself

```sh
go build -o txi .

# peak memory for opening a file and jumping to the end
(sleep 2; printf 'G'; sleep 1; printf 'q') | \
  script -q /dev/null /usr/bin/time -l ./txi test.txt 2>&1 | grep -i 'maximum resident'
```

Divide by 1048576 for MB. Try it against files of very different sizes — the number
barely moves. That's the whole point.

```sh
go test -v ./...
```

The load-bearing test is `TestBufferMatchesFullDecode`: it generates a 20,000-line file
containing empty lines, multi-byte UTF-8, and lines wider than the entire 64KB window,
then compares **every single line** against a plain `bufio.Scanner` decode — reading
forwards, then backwards, then jumping around. If a window boundary ever landed in the
middle of a line, that test fails.

---

## 9. Summary in three sentences

The old version decoded all 202,000 lines into 4-bytes-per-character arrays and held them
forever, so 23MB of file became 130MB of RAM before you'd read a word.

The new version keeps only where each line *starts* (8 bytes each) and reads the actual
text through one reusable 64KB window as you move.

Peak memory went from 130.2MB to 8.8MB — of which 5.2MB is the Go runtime, meaning the
23MB file itself now costs 2.6MB, and stays there however far you scroll.
