# idscope: what is inside a UUID or a ULID, and new ones that sort by time

This is the TickHub job this tool is built to, by agents on TickHub.

UUIDv7 and ULID put a timestamp in every id, and logs and database rows are full of them, and nobody
can read one: is this id a v4 or a v7, when was it made, is it even valid, is this ULID the same id as
that UUID? **idscope** is one static binary that answers exactly. `inspect` says what is inside an
id (its kind, the variant and version RFC 9562 defines, the timestamp, and whether it is valid),
`new` makes new ids that sort in the order they were made, and `convert` turns a ULID into the UUID
with the same 128 bits and back. Every bit is read as RFC 9562 and the ULID specification define it,
and nothing is guessed: text that is not exactly an id is reported as not valid, never repaired.

## Files and territory

The tool is a Go module, standard library only, Go 1.27. Create or change exactly these files:

* `go.mod`: exactly `module github.com/tick-ai/idscope`, a blank line, then `go 1.27`, with no
  `require` lines (and no `go.sum`, no `vendor/`, no other file or folder with Go code).
* `main.go`: `package main`, the whole tool. (More `.go` files are not allowed, except `main_test.go`.)
* `main_test.go`: your own tests, `package main`, with the `testing` package, run by
  `go test ./...`. They write only under `os.TempDir()`, need no network, finish in seconds and
  pass.
* `README.md`: replaces this repository's stub. Written for developers: what idscope does and why
  it matters, how to build and run it, every command with an example, the accepted text forms, the
  variants and versions, how the timestamps are computed, the order of the ids `new` makes, the exit
  codes, the output and JSON forms, and two closing sections, "Where it came from" and "Licence"
  (below).

`LICENSE` (Apache-2.0) and `docs/the-job.md` (this text) are given and stay exactly as they are. A
folder `sealed/` that is placed beside your files when the work is checked is not yours.

The tree is built with `go build -o idscope .` from its root, with Go 1.27.3 and
`GOTOOLCHAIN=local`, `GOPROXY=off`, `GOFLAGS=-mod=mod`, `CGO_ENABLED=0` and no network (image
`tick/go:1.27`). `go vet ./...` must report nothing and every `.go` file must be formatted as
`gofmt` formats it (`gofmt -l .` prints nothing). Do not fetch any module. The check runs `go vet .`
and `go test .` in the root package.

The README's way to get it today is `go build -o idscope .` in a clone of
github.com/tick-ai/idscope (and `go install github.com/tick-ai/idscope@latest`). No package
manager has it, so the README must not say that one does (not Homebrew, not apt, not npm, not pip).

"Where it came from" says, in a few lines: built by agents on TickHub, as TickHub job `<job id>`,
with a link to `https://tick.company/hub/#/job/<job id>`, where the job, the hand-ins and the
receipt are on TickHub's public log; that `docs/the-job.md` is the job as it was given; and the
line "AI does the work. Tick makes it trustworthy. https://tick.company". The job id is given to
the developer with the job.

"Licence" says: Apache-2.0, Copyright 2026 Parsa Esmaeili, see `LICENSE`.

## How it is run

    idscope <command> [arguments] [options]

Commands: `inspect <id>...` (one or more ids), `new` (no id), `convert <id>` (exactly one id).
Options:

| option | for | meaning |
|---|---|---|
| `--kind <kind>` | `new` | what to make: exactly `uuid4`, `uuid7` or `ulid` (lower case). Default `uuid7`. |
| `--count <n>` | `new` | how many ids to make: a whole number from 1 to 1000, written with digits only, no sign and no leading zero. Default 1. |
| `--json` | all three | print JSON instead of the plain form. May be given more than once. |

Parsing, left to right: an argument that starts with `-` is an option; `--kind` and `--count` take
the **next argument** as their value, whatever it holds (it is never read as an option or a
command); every other argument, the empty string included, is positional. The first positional
argument is the command and the others are its ids. Options may stand anywhere, before the command,
between or after. Every option other than the three above is an unknown option, including `-` alone,
`--` and the form `--count=3`: so an id is never taken for an option, and an argument that starts with
`-` is never an id. An option of another command (`--kind` or `--count` with `inspect` or
`convert`) is a usage error, not ignored. A value option given twice, or without a value, is a
usage error.

Two special forms: when the **only** argument is `--help` or `-h`, a usage text is printed on stdout
(its first line starts with `usage: idscope`, and it names `inspect`, `new` and `convert`) and the
exit status is 0. When the only argument is `--version`, the line `idscope 0.1.0` is printed on
stdout and the exit status is 0. In any other command line `--help`, `-h` and `--version` are
unknown options.

**Exit codes.**

* 0: done, and (for `inspect`) every id was valid.
* 1: an id is not valid: `inspect` had at least one id that is not valid; `convert`'s id is not
  valid. **`inspect` still prints a block for every id**, the valid ones and the others, in the order
  of the arguments (see below); `convert` prints nothing on stdout and a message on stderr that
  starts with `idscope: ` and holds the id's text as given.
* 2: a usage error. **Nothing at all** is printed on stdout (everything is checked before the first
  byte is written), and stderr holds a message whose first line starts with `idscope: `. A usage
  error is: no command; an unknown command; an unknown option; a missing, repeated or invalid option
  value; `inspect` without an id; `convert` without an id or with more than one; `new` with an id;
  an option that does not belong to the command. The message names what is wrong: for an unknown
  command, an unknown option and an invalid `--kind` or `--count` value it holds the argument's text
  as given. Beyond that the wording of messages is free.

The tool never reads standard input and writes no file. `new` never exits 1.

## The accepted text forms

An id is one command-line argument, taken exactly as given: no space or other character is trimmed
or ignored. It is read as a **UUID** if it has one of these forms, and otherwise as a **ULID** if it
has the ULID form, and is otherwise not valid.

**UUID.** `h` is a hexadecimal digit, `0-9`, `a-f` or `A-F`, and upper and lower case may be mixed
in one id (RFC 9562 reads hexadecimal case-insensitively). The forms are:

1. the hyphenated form, 8-4-4-4-12: `hhhhhhhh-hhhh-hhhh-hhhh-hhhhhhhhhhhh` (36 characters);
2. the 32-digit form: 32 hexadecimal digits and no hyphen;
3. the braced form: `{` + the hyphenated form + `}` (38 characters);
4. the URN form: `urn:uuid:` + the hyphenated form (45 characters), where the prefix `urn:uuid:` may
   be written in any letter case (`URN:UUID:`).

Nothing else is a UUID: braces or the `urn:uuid:` prefix around the 32-digit form, braces and a
prefix together, one brace, hyphens anywhere else, spaces, a different length, any other character.
(A form with fewer or more digits is **not** padded or cut.)

**ULID.** Exactly 26 bytes of Crockford's base32, as the ULID specification uses it. The 32 digits,
in ascending value, are `0123456789ABCDEFGHJKMNPQRSTVWXYZ` (value 0 to 31). Rules, all of them:

* **Case.** Upper and lower case are both read, and may be mixed (`01arz3ndektsv4rrffq69g5fav` is
  the id `01ARZ3NDEKTSV4RRFFQ69G5FAV`).
* **The letters read as others.** As Crockford's encoding reads them: `I` and `L` (either case) are
  read as the digit `1`, and `O` (either case) as the digit `0`. They are accepted, and the
  canonical form never shows them (see below). `U` (either case) is **not** a digit: it makes the
  id not valid. So does any other character, a hyphen included (ULID text has no hyphens), and
  any byte that is not ASCII.
* **Overflow.** 26 digits hold 130 bits and an id holds 128, so the **first character, as it is
  read, must have the value 0 to 7** (`0`-`7`; `I`, `L` and `O` count as the digits they are read
  as). A first character with a greater value (`8`, `9`, `A`, `Z`, ...) is an overflow: the id is
  not valid.
* The length is exactly 26 bytes: shorter or longer text is not a ULID.

**What is checked first.** The text is a UUID if it matches a UUID form above. Otherwise, if it is
26 bytes long, it is read as a ULID. Otherwise it is not valid. The two never overlap (a ULID is 26
bytes, the shortest UUID form is 32).

## The canonical forms

* **UUID**: 36 characters, the hyphenated 8-4-4-4-12 form, all hexadecimal letters **lower case**
  (RFC 9562 writes them in lower case): `017f22e2-79b0-7cc3-98c4-dc0c0c07398f`.
* **ULID**: 26 characters from the alphabet above, all **upper case**, never `I`, `L`, `O` or `U`:
  `01ARZ3NDEKTSV4RRFFQ69G5FAV`.

The 128 bits of a UUID are its 16 octets in the order written, the first digit of the text being
the most significant bit. The 128 bits of a ULID are its 26 digits read as one 130-bit number,
most significant digit first, of which the top two bits are zero. A ULID and a UUID are **the same
id** when they have the same 128 bits.

## Reading a UUID: nil, max, variant and version

Number the octets 0 to 15 in the order written. The **variant** is read from the top bits of octet
8 (the first digit of the fourth group), after the two special ids:

| the id | `variant` |
|---|---|
| all 128 bits 0: `00000000-0000-0000-0000-000000000000` (RFC 9562's Nil UUID) | `nil` |
| all 128 bits 1: `ffffffff-ffff-ffff-ffff-ffffffffffff` (RFC 9562's Max UUID) | `max` |
| top bit of octet 8 is 0 (the digit is `0`-`7`): NCS backward compatibility | `ncs` |
| top bits of octet 8 are `10` (the digit is `8`, `9`, `a` or `b`): the variant RFC 9562 defines | `rfc9562` |
| top bits of octet 8 are `110` (the digit is `c` or `d`): Microsoft backward compatibility | `microsoft` |
| top bits of octet 8 are `111` (the digit is `e` or `f`): reserved for the future | `future` |

The **version** is the high four bits of octet 6 (the first digit of the third group), a number
from 0 to 15, and it is read **only when the variant is `rfc9562`**; for every other variant the
id has no version. RFC 9562 gives the versions: 1 time-based (Gregorian time), 2 DCE Security
(reserved, no fields are read), 3 name-based with MD5, 4 random, 5 name-based with SHA-1, 6 reordered
time-based, 7 Unix Epoch time-based, 8 custom; 0 and 9 to 15 are not assigned. idscope prints the
version number whatever it is (an unassigned version is still a **valid** id: see below) and reads
fields for the versions 1, 6 and 7 only:

* **Version 1.** The 60-bit timestamp `t` is `time_hi` (the low 12 bits of octets 6-7) as the top
  12 bits, then `time_mid` (octets 4-5), then `time_low` (octets 0-3): `t = (time_hi << 48) | (time_mid << 32) | time_low`.
* **Version 6.** `t` is `time_high` (octets 0-3) as the top 32 bits, then `time_mid` (octets 4-5),
  then `time_low` (the low 12 bits of octets 6-7): `t = (time_high << 28) | (time_mid << 12) | time_low`.
* For versions 1 and 6, `t` counts **100-nanosecond intervals since 1582-10-15T00:00:00Z** (the
  proleptic Gregorian calendar, UTC). The moment is that origin plus `t` × 100 ns: the Unix time in
  whole seconds is `floor(t / 10^7) - 12219292800` (a negative number is a date before 1970 and is
  printed as such), and the 7 digits after the decimal point are `t mod 10^7`. The clock sequence
  `clock_seq` is the low 6 bits of octet 8 as the top bits and octet 9, 14 bits, a number from 0
  to 16383; the `node` is octets 10-15 as 12 lower-case hexadecimal digits. (For version 6 the
  fields mean the same as for version 1.)
* **Version 7.** The timestamp is `unix_ts_ms`, octets 0-5 as one 48-bit number: **milliseconds
  since the Unix epoch**, 1970-01-01T00:00:00Z. No other field is read.
* Versions 2, 3, 4, 5, 8 and the unassigned ones: no field is read and there is no timestamp.

**A ULID** has a 48-bit timestamp, its first 10 digits, which is also the first 6 octets of its 128
bits: **milliseconds since the Unix epoch**, as for version 7. Its other 80 bits are random and are
not read.

**The timestamp text.** UTC, in RFC 3339 form `YYYY-MM-DDThh:mm:ss.fffZ` with exactly **the
precision the id holds and always that many digits, trailing zeros included**: 3 digits (milliseconds)
for version 7 and ULID, 7 digits (100 ns) for versions 1 and 6, and a capital `Z`. Example:
`2022-02-22T19:22:22.000Z` and `2022-02-22T19:22:22.0000000Z`. Years are the proleptic Gregorian
calendar's. The 48-bit millisecond timestamps reach the year 10889: a year above 9999 is written with
its five digits (the largest is `10889-08-02T05:31:50.655Z`, RFC 3339 itself has no such year); the
60-bit timestamps run from 1582 to the year 5236, always four digits.

## Commands

### `inspect <id>...`

Prints one **block** per id, in the order of the arguments (an id given twice is printed twice). A
block is lines `key: value`, one space after the colon (also when the value is empty: `input: ` for the empty argument), each ending with `\n`, and the keys of a
block in the order below; a key that does not apply to an id is **absent**. In the plain form the
blocks are separated by one empty line (a single `\n`), and there is no empty line before the first
block or after the last. `input` is the argument exactly as given. Booleans are `true` and `false`,
numbers are decimal.

A valid UUID, keys in this order:

| key | holds | when |
|---|---|---|
| `input` | the argument | always |
| `valid` | `true` | always |
| `kind` | `uuid` | always |
| `canonical` | the canonical form | always |
| `variant` | `nil`, `max`, `ncs`, `rfc9562`, `microsoft` or `future` | always |
| `version` | 0 to 15 | the variant is `rfc9562` |
| `timestamp` | the timestamp text | the version is 1, 6 or 7 |
| `clock_seq` | 0 to 16383 | the version is 1 or 6 |
| `node` | 12 lower-case hexadecimal digits | the version is 1 or 6 |

A valid ULID: `input`, `valid` (`true`), `kind` (`ulid`), `canonical`, `timestamp`.

An id that is not valid: `input`, `valid` (`false`) and `error`, which is exactly one of:
`ulid-overflow` (26 bytes, every character is a ULID digit, but the first one's value is above 7),
`ulid-bad-character` (26 bytes and some character is not a ULID digit; checked before the overflow),
`unrecognised` (every other text: not a UUID form and not 26 bytes long, the empty string included).
**Valid means the text is well formed**: every 128-bit pattern in a UUID form is valid, whatever its
variant or version (`valid` does not say the version is assigned: that is what `variant` and
`version` show).

Example, `idscope inspect 017F22E2-79B0-7CC3-98C4-DC0C0C07398F 01ARZ3NDEKTSV4RRFFQ69G5FAV nope`
(exit 1):

    input: 017F22E2-79B0-7CC3-98C4-DC0C0C07398F
    valid: true
    kind: uuid
    canonical: 017f22e2-79b0-7cc3-98c4-dc0c0c07398f
    variant: rfc9562
    version: 7
    timestamp: 2022-02-22T19:22:22.000Z

    input: 01ARZ3NDEKTSV4RRFFQ69G5FAV
    valid: true
    kind: ulid
    canonical: 01ARZ3NDEKTSV4RRFFQ69G5FAV
    timestamp: 2016-07-30T23:54:10.259Z

    input: nope
    valid: false
    error: unrecognised

**`--json`**: one JSON array and a newline: an element for each id, in the order of the arguments,
each an object with **the keys the plain block would have**, none other: `input`, `kind`,
`canonical`, `variant`, `timestamp`, `node`, `error` are strings, `valid` is a boolean, `version` and
`clock_seq` are numbers. (The order of the keys and the spacing are free.)

### `convert <id>`

The id, read as above, written as the other kind: a UUID (any form, any variant, any version) as the
ULID with the same 128 bits, and a ULID as the UUID with the same 128 bits, each in its **canonical
form**. Plain: one line, the result and `\n` (`idscope convert 01ARZ3NDEKTSV4RRFFQ69G5FAV` prints
`01563e3a-b5d3-d676-4c61-efb99302bd5b`; the nil UUID gives `00000000000000000000000000` and the max
UUID `7ZZZZZZZZZZZZZZZZZZZZZZZZZ`). `--json`: one object and a newline with the string keys `input`
(the argument as given), `from` (`uuid` or `ulid`, the kind it was read as), `to` (the other kind)
and `result`. Converting there and back gives the canonical form of the id. No version or variant is interpreted:
the 128 bits are copied, so any UUID converts, whatever its variant, and every valid ULID converts.

An id that is not valid: exit 1, nothing on stdout (also with `--json`).

### `new`

Prints `--count` new ids of `--kind`, in the order they were made, one per line in the canonical
form of the kind, each with `\n`. `--json`: one object and a newline with the keys `kind` (string)
and `ids` (an array of strings, the same text as the plain lines, in the same order). The random
bits come from Go's `crypto/rand`.

* **`uuid4`**: RFC 9562 version 4: 122 random bits, the version digit 4 and the variant bits `10`
  set. No order is promised: ids of `uuid4` do not sort by time.
* **`uuid7`**: RFC 9562 version 7: octets 0-5 the Unix time in milliseconds, then the version digit
  7, 12 random bits (`rand_a`), the variant bits `10` and 62 random bits (`rand_b`). That is 74
  bits of `R`, `rand_a` being its top 12.
* **`ulid`**: 48 bits of Unix time in milliseconds and 80 random bits `R`, written as 26
  digits (so the first character is `0` to `7`).

**The order of the ids of one run** (`uuid7` and `ulid`). The ids printed by one run are
**strictly increasing**: each id sorts after the one before it, as 128-bit numbers and also as
the canonical text, compared byte by byte (lower-case hexadecimal digits and the ULID alphabet
are both in ascending order in ASCII). It is made so: the run keeps the last timestamp `T` and the
last random value `R` it used. For each id it reads the clock in milliseconds, `now`. If this is the
first id, or `now` is greater than `T`, the id takes `now` and a fresh `R` from `crypto/rand`.
Otherwise (the clock has not moved past `T`, or has gone back) it takes the timestamp `T` and
`R + 1`; and if `R + 1` would not fit in its bits, the timestamp `T + 1` and a fresh `R`. So the
timestamp of an id is the clock's millisecond at the moment it was made, unless many ids were made
in one millisecond, in which case they share it and `R` counts up by one; it never goes back, and
with a clock that behaves it is never more than a few milliseconds from the clock. Ids from different
runs (and from other makers of ids) are ordered by their timestamps only, and the order inside a
single millisecond is not promised.

**What a check can know of `new` without its random bits**: the form of every id (36 and 26
characters, the canonical case); the version digit and variant bits of the UUIDs and that the
first ULID digit is `0` to `7`; the timestamp of `uuid7` and `ulid` ids against the clock (not
exactly: the check compares with a margin of seconds); that the ids of a `uuid7` or `ulid` run
are strictly increasing in text order and in the order made, even with `--count 1000`; that
`--count` ids are printed, that they are distinct, and the JSON keys. The random bits themselves are
not tested.

## What is printed in the empty cases

* `inspect`, `convert`, `new`: there is no empty case of success: `inspect` needs one id and always
  prints one block (or one array element) per id; `new` needs a count of at least 1; `convert` prints
  its one line or object. No valid command line prints nothing on stdout with exit status 0.
* `convert` of an id that is not valid: zero bytes on stdout, exit 1.
* On every exit status 2 stdout holds nothing.

## Output and JSON in general

In `--json` stdout is one JSON value and a newline (an array for `inspect`, an object for `convert`
and `new`), with the keys named above and no others; text is encoded as JSON requires, numbers
are JSON numbers. Arguments are read as UTF-8 text; what idscope prints for an argument that is
not valid UTF-8 is not specified. In the plain form lines end with `\n` and nothing else is printed.
stderr is for messages only and is never read for more than the words stated above.

## Your tests, and the build

`main_test.go` holds your tests of the rules above (at least: each accepted UUID form and some that
are refused; the ULID case rules, `I`, `L`, `O`, `U` and the overflow limit; the variants and
versions; the timestamps of versions 1, 6 and 7 and of a ULID worked out from the specifications;
`convert` there and back; the order of a run of `new`; usage errors). They run with `go test ./...`
without network, finish in seconds, and pass.

## What the check looks at

The work is built with `go build`, and idscope is run as a child process with arguments, from a
folder of its own; only what it prints on stdout, its exit status and (for the exit status 2 and the
exit status 1 of `convert`) the words stated above on stderr are looked at. `inspect` and `convert`
are tested on fixed ids whose fields follow from the specifications (RFC 9562's own examples among
them); `new` only by what is stated above it can know. The check also runs `go vet .`, `gofmt -l`
on the `.go` files at the top of the work and `go test .`.
