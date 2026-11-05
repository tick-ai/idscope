// idscope reads what is inside a UUID or a ULID, makes new ids that sort by
// time, and converts between a ULID and the UUID with the same 128 bits.
package main

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"time"
)

const version = "0.1.0"

const usageText = `usage: idscope <command> [arguments] [options]

commands:
  inspect <id>...   say what is inside each UUID or ULID (exit 1 if any is not valid)
  new               make new ids that sort in the order they were made
  convert <id>      write a ULID as the UUID with the same 128 bits, and back

options:
  --kind <kind>     for new: uuid4, uuid7 or ulid (default uuid7)
  --count <n>       for new: how many ids, 1 to 1000 (default 1)
  --json            print JSON instead of the plain form
  --help, -h        print this text (only as the sole argument)
  --version         print the version (only as the sole argument)
`

// crockford is the ULID alphabet, in ascending value.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// gregorianToUnix is the number of seconds from 1582-10-15 to 1970-01-01.
const gregorianToUnix = 12219292800

// id128 holds the 128 bits of an id, most significant octet first.
type id128 [16]byte

// digitValue maps a byte to its ULID digit value, or -1.
var digitValue = func() (t [256]int8) {
	for i := range t {
		t[i] = -1
	}
	for i := 0; i < len(crockford); i++ {
		c := crockford[i]
		t[c] = int8(i)
		if c >= 'A' && c <= 'Z' {
			t[c+'a'-'A'] = int8(i)
		}
	}
	for _, c := range "IiLl" {
		t[c] = 1
	}
	for _, c := range "Oo" {
		t[c] = 0
	}
	return t
}()

func hexVal(c byte) int {
	switch {
	case c >= '0' && c <= '9':
		return int(c - '0')
	case c >= 'a' && c <= 'f':
		return int(c-'a') + 10
	case c >= 'A' && c <= 'F':
		return int(c-'A') + 10
	}
	return -1
}

func fromHex32(s string) (u id128, ok bool) {
	if len(s) != 32 {
		return id128{}, false
	}
	for i := 0; i < 16; i++ {
		h, l := hexVal(s[2*i]), hexVal(s[2*i+1])
		if h < 0 || l < 0 {
			return id128{}, false
		}
		u[i] = byte(h<<4 | l)
	}
	return u, true
}

func fromHyphenated(s string) (u id128, ok bool) {
	if len(s) != 36 {
		return id128{}, false
	}
	n, hi := 0, 0
	for i := 0; i < 36; i++ {
		c := s[i]
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return id128{}, false
			}
			continue
		}
		v := hexVal(c)
		if v < 0 {
			return id128{}, false
		}
		if n%2 == 0 {
			hi = v
		} else {
			u[n/2] = byte(hi<<4 | v)
		}
		n++
	}
	return u, true
}

// hasURNPrefix reports whether s starts with "urn:uuid:" in any ASCII case.
func hasURNPrefix(s string) bool {
	const p = "urn:uuid:"
	if len(s) < len(p) {
		return false
	}
	for i := 0; i < len(p); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		if c != p[i] {
			return false
		}
	}
	return true
}

// parseUUID reads one of the four accepted UUID forms.
func parseUUID(s string) (id128, bool) {
	switch len(s) {
	case 32:
		return fromHex32(s)
	case 36:
		return fromHyphenated(s)
	case 38:
		if s[0] == '{' && s[37] == '}' {
			return fromHyphenated(s[1:37])
		}
	case 45:
		if hasURNPrefix(s) {
			return fromHyphenated(s[9:])
		}
	}
	return id128{}, false
}

// parseULID reads a 26-byte ULID. The error is "" or one of the error names.
func parseULID(s string) (u id128, errName string) {
	if len(s) != 26 {
		return u, "unrecognised"
	}
	var v [26]int8
	for i := 0; i < 26; i++ {
		v[i] = digitValue[s[i]]
		if v[i] < 0 {
			return id128{}, "ulid-bad-character"
		}
	}
	if v[0] > 7 {
		return id128{}, "ulid-overflow"
	}
	var hi, lo uint64
	for i := 0; i < 26; i++ {
		hi = hi<<5 | lo>>59
		lo = lo<<5 | uint64(v[i])
	}
	binary.BigEndian.PutUint64(u[0:8], hi)
	binary.BigEndian.PutUint64(u[8:16], lo)
	return u, ""
}

// parsed is an id read from text.
type parsed struct {
	u    id128
	kind string // "uuid" or "ulid"; "" when not valid
	err  string // error name when not valid
}

func parseID(s string) parsed {
	if u, ok := parseUUID(s); ok {
		return parsed{u: u, kind: "uuid"}
	}
	u, e := parseULID(s)
	if e != "" {
		return parsed{err: e}
	}
	return parsed{u: u, kind: "ulid"}
}

func (u id128) uuidString() string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", u[0:4], u[4:6], u[6:8], u[8:10], u[10:16])
}

func (u id128) ulidString() string {
	hi := binary.BigEndian.Uint64(u[0:8])
	lo := binary.BigEndian.Uint64(u[8:16])
	var out [26]byte
	for i := 0; i < 26; i++ {
		s := uint(5 * (25 - i))
		var v uint64
		if s >= 64 {
			v = hi >> (s - 64)
		} else {
			v = lo>>s | hi<<(64-s)
		}
		out[i] = crockford[v&31]
	}
	return string(out[:])
}

// unixMillis is the first 48 bits: the timestamp of version 7 and of a ULID.
func (u id128) unixMillis() uint64 {
	var ms uint64
	for i := 0; i < 6; i++ {
		ms = ms<<8 | uint64(u[i])
	}
	return ms
}

func (u id128) variant() string {
	zero, ones := true, true
	for _, b := range u {
		if b != 0 {
			zero = false
		}
		if b != 0xff {
			ones = false
		}
	}
	switch {
	case zero:
		return "nil"
	case ones:
		return "max"
	case u[8]&0x80 == 0:
		return "ncs"
	case u[8]&0xc0 == 0x80:
		return "rfc9562"
	case u[8]&0xe0 == 0xc0:
		return "microsoft"
	}
	return "future"
}

// formatTime writes a UTC RFC 3339 time with exactly digits fractional digits.
func formatTime(unixSec int64, frac uint64, digits int) string {
	t := time.Unix(unixSec, 0).UTC()
	y, m, d := t.Date()
	return fmt.Sprintf("%04d-%02d-%02dT%02d:%02d:%02d.%0*dZ", y, int(m), d, t.Hour(), t.Minute(), t.Second(), digits, frac)
}

func millisText(ms uint64) string {
	return formatTime(int64(ms/1000), ms%1000, 3)
}

func gregorianText(t uint64) string {
	return formatTime(int64(t/10000000)-gregorianToUnix, t%10000000, 7)
}

// field is one key of an inspect block.
type field struct {
	key string
	val any // string, bool or int
}

func inspectFields(input string) (fs []field, valid bool) {
	p := parseID(input)
	if p.kind == "" {
		return []field{{"input", input}, {"valid", false}, {"error", p.err}}, false
	}
	u := p.u
	fs = []field{{"input", input}, {"valid", true}, {"kind", p.kind}}
	if p.kind == "ulid" {
		fs = append(fs, field{"canonical", u.ulidString()}, field{"timestamp", millisText(u.unixMillis())})
		return fs, true
	}
	variant := u.variant()
	fs = append(fs, field{"canonical", u.uuidString()}, field{"variant", variant})
	if variant != "rfc9562" {
		return fs, true
	}
	ver := int(u[6] >> 4)
	fs = append(fs, field{"version", ver})
	low12 := uint64(u[6]&0x0f)<<8 | uint64(u[7])
	mid := uint64(u[4])<<8 | uint64(u[5])
	top32 := uint64(binary.BigEndian.Uint32(u[0:4]))
	var ts string
	switch ver {
	case 1:
		ts = gregorianText(low12<<48 | mid<<32 | top32)
	case 6:
		ts = gregorianText(top32<<28 | mid<<12 | low12)
	case 7:
		ts = millisText(u.unixMillis())
	}
	if ts != "" {
		fs = append(fs, field{"timestamp", ts})
	}
	if ver == 1 || ver == 6 {
		seq := int(u[8]&0x3f)<<8 | int(u[9])
		fs = append(fs, field{"clock_seq", seq}, field{"node", fmt.Sprintf("%x", u[10:16])})
	}
	return fs, true
}

func plainValue(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case bool:
		return strconv.FormatBool(x)
	case int:
		return strconv.Itoa(x)
	}
	return ""
}

func jsonValue(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func writeJSONObject(buf *bytes.Buffer, fs []field) {
	buf.WriteByte('{')
	for i, f := range fs {
		if i > 0 {
			buf.WriteByte(',')
		}
		buf.Write(jsonValue(f.key))
		buf.WriteByte(':')
		buf.Write(jsonValue(f.val))
	}
	buf.WriteByte('}')
}

// maker makes the ids of one run of uuid7 or ulid in strictly increasing order.
type maker struct {
	bits  uint // width of R: 74 for uuid7, 80 for ulid
	now   func() uint64
	fresh func(bits uint) (hi, lo uint64)
	have  bool
	t     uint64
	hi    uint64 // R's bits above 64
	lo    uint64 // R's low 64 bits
}

func (m *maker) next() (t, hi, lo uint64) {
	now := m.now()
	if !m.have || now > m.t {
		m.t = now
		m.hi, m.lo = m.fresh(m.bits)
	} else {
		m.lo++
		if m.lo == 0 {
			m.hi++
		}
		if m.hi>>(m.bits-64) != 0 {
			m.t++
			m.hi, m.lo = m.fresh(m.bits)
		}
	}
	m.have = true
	return m.t, m.hi, m.lo
}

func randomBytes(b []byte) {
	if _, err := rand.Read(b); err != nil {
		panic("idscope: no random bits: " + err.Error())
	}
}

func randomR(bits uint) (hi, lo uint64) {
	var b [16]byte
	randomBytes(b[:])
	hi = binary.BigEndian.Uint64(b[0:8]) & (uint64(1)<<(bits-64) - 1)
	lo = binary.BigEndian.Uint64(b[8:16])
	return hi, lo
}

func clockMillis() uint64 {
	return uint64(time.Now().UnixMilli()) & (1<<48 - 1)
}

func putMillis(u *id128, t uint64) {
	for k := 0; k < 6; k++ {
		u[k] = byte(t >> (8 * (5 - k)))
	}
}

func buildULID(t, hi, lo uint64) id128 {
	var u id128
	putMillis(&u, t)
	binary.BigEndian.PutUint16(u[6:8], uint16(hi))
	binary.BigEndian.PutUint64(u[8:16], lo)
	return u
}

func buildUUID7(t, hi, lo uint64) id128 {
	var u id128
	putMillis(&u, t)
	randA := hi<<2 | lo>>62
	randB := lo & (1<<62 - 1)
	u[6] = 0x70 | byte(randA>>8)
	u[7] = byte(randA)
	u[8] = 0x80 | byte(randB>>56)
	for k := 0; k < 7; k++ {
		u[9+k] = byte(randB >> (8 * (6 - k)))
	}
	return u
}

func newUUID4() id128 {
	var u id128
	randomBytes(u[:])
	u[6] = u[6]&0x0f | 0x40
	u[8] = u[8]&0x3f | 0x80
	return u
}

func makeIDs(kind string, count int) []string {
	ids := make([]string, 0, count)
	switch kind {
	case "uuid4":
		for i := 0; i < count; i++ {
			ids = append(ids, newUUID4().uuidString())
		}
	case "uuid7":
		m := &maker{bits: 74, now: clockMillis, fresh: randomR}
		for i := 0; i < count; i++ {
			t, hi, lo := m.next()
			ids = append(ids, buildUUID7(t, hi, lo).uuidString())
		}
	case "ulid":
		m := &maker{bits: 80, now: clockMillis, fresh: randomR}
		for i := 0; i < count; i++ {
			t, hi, lo := m.next()
			ids = append(ids, buildULID(t, hi, lo).ulidString())
		}
	}
	return ids
}

func validCount(s string) (int, bool) {
	if len(s) < 1 || len(s) > 4 || s[0] == '0' {
		return 0, false
	}
	n := 0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, false
		}
		n = n*10 + int(s[i]-'0')
	}
	return n, n >= 1 && n <= 1000
}

func usageErrors(w io.Writer, errs []string) int {
	for _, e := range errs {
		fmt.Fprintf(w, "idscope: %s\n", e)
	}
	return 2
}

func realMain(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 {
		switch args[0] {
		case "--help", "-h":
			io.WriteString(stdout, usageText)
			return 0
		case "--version":
			io.WriteString(stdout, "idscope "+version+"\n")
			return 0
		}
	}

	var (
		asJSON            bool
		kind, count       string
		kindSet, countSet bool
		pos, errs         []string
	)
	for i := 0; i < len(args); i++ {
		a := args[i]
		if !strings.HasPrefix(a, "-") {
			pos = append(pos, a)
			continue
		}
		switch a {
		case "--json":
			asJSON = true
		case "--kind", "--count":
			if i+1 >= len(args) {
				errs = append(errs, "option "+a+" needs a value")
				continue
			}
			value := args[i+1]
			i++
			set, dst := &kindSet, &kind
			if a == "--count" {
				set, dst = &countSet, &count
			}
			if *set {
				errs = append(errs, "option "+a+" given more than once")
				continue
			}
			*set, *dst = true, value
		default:
			errs = append(errs, `unknown option "`+a+`"`)
		}
	}

	var cmd string
	var ids []string
	if len(pos) == 0 {
		errs = append(errs, "no command: use inspect, new or convert (see --help)")
	} else {
		cmd, ids = pos[0], pos[1:]
		switch cmd {
		case "inspect", "convert":
			if len(ids) == 0 {
				errs = append(errs, cmd+" needs an id")
			}
			if cmd == "convert" && len(ids) > 1 {
				errs = append(errs, "convert takes exactly one id, got "+strconv.Itoa(len(ids)))
			}
			if kindSet {
				errs = append(errs, "option --kind belongs to new, not "+cmd)
			}
			if countSet {
				errs = append(errs, "option --count belongs to new, not "+cmd)
			}
		case "new":
			if len(ids) > 0 {
				errs = append(errs, `new takes no id, got "`+ids[0]+`"`)
			}
			if kindSet && kind != "uuid4" && kind != "uuid7" && kind != "ulid" {
				errs = append(errs, `invalid --kind "`+kind+`": use uuid4, uuid7 or ulid`)
			}
			if countSet {
				if _, ok := validCount(count); !ok {
					errs = append(errs, `invalid --count "`+count+`": use a whole number from 1 to 1000`)
				}
			}
		default:
			errs = append(errs, `unknown command "`+cmd+`"`)
		}
	}
	if len(errs) > 0 {
		return usageErrors(stderr, errs)
	}

	var buf bytes.Buffer
	status := 0
	switch cmd {
	case "inspect":
		if asJSON {
			buf.WriteByte('[')
		}
		for i, in := range ids {
			fs, valid := inspectFields(in)
			if !valid {
				status = 1
			}
			if asJSON {
				if i > 0 {
					buf.WriteByte(',')
				}
				writeJSONObject(&buf, fs)
				continue
			}
			if i > 0 {
				buf.WriteByte('\n')
			}
			for _, f := range fs {
				buf.WriteString(f.key + ": " + plainValue(f.val) + "\n")
			}
		}
		if asJSON {
			buf.WriteString("]\n")
		}
	case "convert":
		p := parseID(ids[0])
		if p.kind == "" {
			fmt.Fprintf(stderr, "idscope: not a valid id: %s\n", ids[0])
			return 1
		}
		from, to, result := "uuid", "ulid", p.u.ulidString()
		if p.kind == "ulid" {
			from, to, result = "ulid", "uuid", p.u.uuidString()
		}
		if asJSON {
			writeJSONObject(&buf, []field{{"input", ids[0]}, {"from", from}, {"to", to}, {"result", result}})
			buf.WriteByte('\n')
		} else {
			buf.WriteString(result + "\n")
		}
	case "new":
		k, n := "uuid7", 1
		if kindSet {
			k = kind
		}
		if countSet {
			n, _ = validCount(count)
		}
		out := makeIDs(k, n)
		if asJSON {
			b, _ := json.Marshal(struct {
				Kind string   `json:"kind"`
				IDs  []string `json:"ids"`
			}{k, out})
			buf.Write(b)
			buf.WriteByte('\n')
		} else {
			for _, s := range out {
				buf.WriteString(s + "\n")
			}
		}
	}
	stdout.Write(buf.Bytes())
	return status
}

func main() {
	os.Exit(realMain(os.Args[1:], os.Stdout, os.Stderr))
}
