# idscope

**idscope says what is inside a UUID or a ULID (kind, variant, version, timestamp, valid or not), makes new ids that sort in the order they were made, and turns a ULID into the UUID with the same 128 bits and back.**

    idscope <command> [arguments] [options]

```
$ idscope inspect 017F22E2-79B0-7CC3-98C4-DC0C0C07398F 01ARZ3NDEKTSV4RRFFQ69G5FAV nope
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
$ echo $?
1

$ idscope convert 01ARZ3NDEKTSV4RRFFQ69G5FAV
01563e3a-b5d3-d676-4c61-efb99302bd5b
```

## Why it matters

UUIDv7 and ULID put a timestamp in every id, and logs and database rows are full of them, and nobody can
read one: is this id a v4 or a v7, when was it made, is it even valid, is this ULID the same id as that
UUID? idscope is one static binary, Go standard library only, that answers exactly. Every bit is read as
RFC 9562 and the ULID specification define it, and nothing is guessed: text that is not exactly an id is
reported as not valid, never repaired.

## Build and run

You need Go 1.27. In a clone of github.com/tick-ai/idscope:

    go build -o idscope .

or

    go install github.com/tick-ai/idscope@latest

No package manager has idscope today. It has no dependency outside the Go standard library, reads no
standard input and writes no file.

    idscope <command> [arguments] [options]

| command | what it does |
|---|---|
| `inspect <id>...` | one or more ids: what is inside each |
| `new` | makes new ids |
| `convert <id>` | exactly one id: the same 128 bits as the other kind |

| option | for | meaning |
|---|---|---|
| `--kind <kind>` | `new` | what to make: exactly `uuid4`, `uuid7` or `ulid` (lower case). Default `uuid7`. |
| `--count <n>` | `new` | how many ids: a whole number from 1 to 1000, digits only, no sign, no leading zero. Default 1. |
| `--json` | all three | print JSON instead of the plain form. May be given more than once. |

Command line rules, read left to right: an argument that starts with `-` is an option. `--kind` and
`--count` take the **next argument** as their value, whatever it holds. Every other argument, the empty
string included, is positional: the first is the command and the rest are its ids. Options may stand
anywhere. Every other option is an unknown option, including `-`, `--` and `--count=3`, so an argument
that starts with `-` is never an id. `--kind` or `--count` with `inspect` or `convert` is a usage error,
and so is a value option given twice or without a value. `--help` (or `-h`) and `--version` work only as
the sole argument; anywhere else they are unknown options.

    $ idscope --version
    idscope 0.1.0

## Commands

### inspect

    idscope inspect <id>... [--json]

Prints one block per id, in the order given (an id given twice is printed twice). A block is lines of
`key: value`; a key that does not apply to an id is absent. Blocks are separated by one empty line.
`input` is the argument exactly as given. Exit status 1 if any id is not valid, and every id is still
printed.

A valid UUID has these keys, in this order:

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
| `node` | 12 lower-case hex digits | the version is 1 or 6 |

A valid ULID has `input`, `valid` (`true`), `kind` (`ulid`), `canonical` and `timestamp`.

An id that is not valid has `input`, `valid` (`false`) and `error`, exactly one of:

* `ulid-bad-character`: 26 bytes and some character is not a ULID digit (checked before the overflow);
* `ulid-overflow`: 26 bytes, every character a ULID digit, but the first one's value is above 7;
* `unrecognised`: any other text, the empty string included.

Valid means well formed: every 128-bit pattern in a UUID form is valid, whatever its variant or
version. An unassigned version is still a valid id; `variant` and `version` show what it is.

A UUID version 1 from RFC 9562, with a date and a node:

```
$ idscope inspect C232AB00-9414-11EC-B3C8-9E6BDECED846
input: C232AB00-9414-11EC-B3C8-9E6BDECED846
valid: true
kind: uuid
canonical: c232ab00-9414-11ec-b3c8-9e6bdeced846
variant: rfc9562
version: 1
timestamp: 2022-02-22T19:22:22.0000000Z
clock_seq: 13256
node: 9e6bdeced846
```

### convert

    idscope convert <id> [--json]

Writes a UUID as the ULID with the same 128 bits, and a ULID as the UUID with the same 128 bits, each in
its canonical form, one line. No variant or version is interpreted: the 128 bits are copied, so any UUID
converts, and every valid ULID converts. There and back gives the canonical form of the id.

```
$ idscope convert 01ARZ3NDEKTSV4RRFFQ69G5FAV
01563e3a-b5d3-d676-4c61-efb99302bd5b
$ idscope convert 00000000-0000-0000-0000-000000000000
00000000000000000000000000
$ idscope convert ffffffff-ffff-ffff-ffff-ffffffffffff
7ZZZZZZZZZZZZZZZZZZZZZZZZZ
```

With `--json`: one object with the string keys `input` (as given), `from` (`uuid` or `ulid`, what it was
read as), `to` (the other kind) and `result`. An id that is not valid: exit status 1, nothing on stdout
(also with `--json`) and an `idscope: ` message on stderr that holds the id's text.

### new

    idscope new [--kind uuid4|uuid7|ulid] [--count 1..1000] [--json]

Prints `--count` new ids of `--kind`, in the order they were made, one per line in the canonical form of
the kind. With `--json`: one object with `kind` and `ids` (the same strings, in the same order). The
random bits come from Go's `crypto/rand`. `new` never exits 1.

* `uuid4`: RFC 9562 version 4: 122 random bits, version digit 4, variant bits `10`. They do not sort by time.
* `uuid7` (the default): RFC 9562 version 7: the Unix time in milliseconds, version digit 7, 12 random
  bits (`rand_a`), variant bits `10`, 62 random bits (`rand_b`).
* `ulid`: 48 bits of Unix time in milliseconds and 80 random bits, written as 26 digits (the first is `0` to `7`).

```
$ idscope new --kind ulid --count 3        # one id per line, e.g. 01ARZ3NDEKTSV4RRFFQ69G5FAV
$ idscope new --json --kind uuid4          # {"kind":"uuid4","ids":["..."]}
```

#### The order of the ids of one run

The ids of `uuid7` and `ulid` printed by one run are **strictly increasing**: each sorts after the one
before it, as a 128-bit number and as canonical text compared byte by byte, even with `--count 1000`.
The run keeps the last timestamp `T` and the last random value `R` it used (74 bits for `uuid7`, 80 for
`ulid`). For each id it reads the clock in milliseconds, `now`:

* the first id, or `now` greater than `T`: the id takes `now` and a fresh `R` from `crypto/rand`;
* otherwise (the clock has not moved past `T`, or went back): the id takes `T` and `R + 1`; and if
  `R + 1` would not fit in its bits, `T + 1` and a fresh `R`.

So the timestamp of an id is the clock's millisecond when it was made, unless many ids were made in one
millisecond, in which case they share it and `R` counts up by one. It never goes back. Ids from different
runs, or from other makers of ids, are ordered by their timestamps only; the order inside one
millisecond is not promised. `uuid4` ids are not ordered.

## The accepted text forms

An id is one command-line argument, taken exactly as given: no space or other character is trimmed or
ignored. It is a UUID if it has one of the forms below, otherwise a ULID if it has the ULID form,
otherwise not valid.

**UUID.** `h` is a hexadecimal digit of either case, mixed freely.

| form | example |
|---|---|
| hyphenated, 8-4-4-4-12 | `017f22e2-79b0-7cc3-98c4-dc0c0c07398f` |
| 32 digits | `017f22e279b07cc398c4dc0c0c07398f` |
| braced | `{017f22e2-79b0-7cc3-98c4-dc0c0c07398f}` |
| URN (the prefix `urn:uuid:` in any letter case) | `urn:uuid:017f22e2-79b0-7cc3-98c4-dc0c0c07398f` |

Nothing else is a UUID: braces or the prefix around the 32-digit form, braces and a prefix together, one
brace, hyphens anywhere else, spaces, another length, any other character. A form with fewer or more
digits is not padded or cut.

**ULID.** Exactly 26 bytes of Crockford's base32. The digits in ascending value are
`0123456789ABCDEFGHJKMNPQRSTVWXYZ`.

* Upper and lower case are both read and may be mixed.
* `I` and `L` (either case) are read as `1`, and `O` (either case) as `0`; the canonical form never shows them.
* `U` is not a digit, and neither is any other character, a hyphen or a non-ASCII byte included: the id is not valid.
* 26 digits hold 130 bits and an id holds 128, so the first character, as read, must be `0` to `7`. `8`, `9`, `A`, `Z`... overflow.
* The length is exactly 26 bytes.

The two forms never overlap: a ULID is 26 bytes and the shortest UUID form is 32.

**Canonical forms.** UUID: 36 characters, hyphenated, lower case. ULID: 26 characters of the alphabet
above, upper case. A ULID and a UUID are the same id when they have the same 128 bits: a UUID's 16 octets
in the order written, a ULID's 26 digits read as one 130-bit number whose top two bits are zero.

## Variants and versions

The variant is read from octet 8 (the first digit of the fourth group), after the two special ids:

| the id | `variant` |
|---|---|
| all 128 bits 0 (RFC 9562's Nil UUID) | `nil` |
| all 128 bits 1 (RFC 9562's Max UUID) | `max` |
| top bit of octet 8 is 0 (digit `0`-`7`): NCS backward compatibility | `ncs` |
| top bits `10` (digit `8`, `9`, `a`, `b`): the variant RFC 9562 defines | `rfc9562` |
| top bits `110` (digit `c`, `d`): Microsoft backward compatibility | `microsoft` |
| top bits `111` (digit `e`, `f`): reserved for the future | `future` |

The version is the high four bits of octet 6 (the first digit of the third group), 0 to 15, and it is
read **only when the variant is `rfc9562`**. RFC 9562 gives: 1 time-based (Gregorian time), 2 DCE
Security, 3 name-based with MD5, 4 random, 5 name-based with SHA-1, 6 reordered time-based, 7 Unix Epoch
time-based, 8 custom; 0 and 9 to 15 are not assigned. idscope prints the number whatever it is and reads
fields for versions 1, 6 and 7 only.

## How the timestamps are computed

* **Version 1.** The 60-bit `t` is `time_hi` (low 12 bits of octets 6-7) as the top 12 bits, then
  `time_mid` (octets 4-5), then `time_low` (octets 0-3).
* **Version 6.** `t` is `time_high` (octets 0-3) as the top 32 bits, then `time_mid` (octets 4-5), then
  `time_low` (low 12 bits of octets 6-7).
* For versions 1 and 6, `t` counts 100-nanosecond intervals since 1582-10-15T00:00:00Z (proleptic
  Gregorian calendar, UTC). The Unix time in whole seconds is `floor(t / 10^7) - 12219292800` (negative
  for a date before 1970) and the 7 digits after the point are `t mod 10^7`. `clock_seq` is the low 6
  bits of octet 8 as the top bits and octet 9 (0 to 16383); `node` is octets 10-15 as 12 lower-case hex digits.
* **Version 7** and **ULID.** The first 48 bits (octets 0-5; for a ULID, its first 10 digits) are
  milliseconds since 1970-01-01T00:00:00Z. No other field is read; the rest of a ULID is random.
* Versions 2, 3, 4, 5, 8 and the unassigned ones have no timestamp.

The text is UTC in the form `YYYY-MM-DDThh:mm:ss.fffZ`, with exactly the precision the id holds and
always that many digits, trailing zeros included: 3 digits for version 7 and ULID, 7 for versions 1 and
6. The largest 48-bit millisecond timestamp is `10889-08-02T05:31:50.655Z` (a year above 9999 is written
with five digits); the largest 60-bit one is `5236-03-31T21:21:00.6846975Z`.

## Exit codes

| status | meaning |
|---|---|
| 0 | done, and for `inspect` every id was valid |
| 1 | an id is not valid: `inspect` had at least one (it still prints a block for every id); `convert`'s id (nothing on stdout, a message on stderr) |
| 2 | a usage error: nothing at all on stdout, and a message on stderr whose first line starts with `idscope: ` |

A usage error is: no command; an unknown command; an unknown option; a missing, repeated or invalid
option value; `inspect` without an id; `convert` without an id or with more than one; `new` with an id;
an option that does not belong to the command. Everything is checked before the first byte is written.

## Output and JSON

In the plain form lines end with `\n` and nothing else is printed. With `--json`, stdout is one JSON value
and a newline: an array for `inspect` (an element per id, in order), an object for `convert` and `new`.

* `inspect` elements have the keys the plain block would have and no others: `input`, `kind`,
  `canonical`, `variant`, `timestamp`, `node` and `error` are strings, `valid` is a boolean, `version`
  and `clock_seq` are numbers.
* `convert`: `input`, `from`, `to`, `result`, all strings.
* `new`: `kind` (string) and `ids` (array of strings).

```
$ idscope inspect --json nope
[{"input":"nope","valid":false,"error":"unrecognised"}]
```

Arguments are read as UTF-8 text; what idscope prints for an argument that is not valid UTF-8 is not
specified.

## Where it came from

Built by agents on TickHub, as TickHub job `b814eb08-4c23-4c6d-a692-9751c0830c69`. The job, the hand-ins
and the receipt are on TickHub's public log: https://tick.company/hub/#/job/b814eb08-4c23-4c6d-a692-9751c0830c69.
`docs/the-job.md` is the job as it was given.

AI does the work. Tick makes it trustworthy. https://tick.company

## Licence

Apache-2.0, Copyright 2026 Parsa Esmaeili, see `LICENSE`.
