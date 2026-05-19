package core

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"slices"
	"time"
)

const SECTION_SIZE = 10000

type circularBuffer struct {
	rPos int // pointer to read location
	wPos int // pointer to write location

	// tracks how much we've read that hasn't been compensated by writing, cannot exceed SECTION_SIZE
	// must always be positive, we cannot write over more sections than have been read, and we cannot read over more
	// SECTION_SIZE points before any writing is done
	rwDelta int

	buf []byte
}

func newCircularBuffer() *circularBuffer {
	return &circularBuffer{
		rPos:    0,
		wPos:    0,
		rwDelta: SECTION_SIZE, // buffer starts in completely drained state, requires a write to start working
		buf:     make([]byte, SECTION_SIZE),
	}
}

func (b *circularBuffer) get(amount int) []byte {
	// However much we have to read that hasn't been read yet
	// I just don't want to return an error here, correcteness will be enforced
	if amount > (len(b.buf) - b.rwDelta) {
		return nil
	}

	start := b.rPos
	end := start + amount
	wrapAround := false
	if end > len(b.buf) {
		wrapAround = true
	}

	if !wrapAround {
		if end == len(b.buf) {
			b.rPos = 0
		} else {
			b.rPos = end
		}
		b.rwDelta += amount
		return slices.Clone(b.buf[start : (start)+amount])
	}

	// update end, first end - len of buffer indicates the overflow amount
	end = end - len(b.buf)

	part := b.buf[start:]
	b.rPos = end
	b.rwDelta += amount
	return append(part, b.buf[:end]...)
}

func (b *circularBuffer) Write(data []byte) (int, error) {
	// Would overwrite sections that haven't yet been read
	if len(data) > b.rwDelta {
		return 0, errors.New("attempt to overwrite not yet read parts") // no-op
	}

	for i := range len(data) {
		b.buf[b.wPos] = data[i]
		b.wPos++
		if b.wPos == len(b.buf) {
			b.wPos = 0
		}
	}

	b.rwDelta -= len(data)
	return len(data), nil
}

// reports the amount of ab
func (b *circularBuffer) readAvailable() int {
	return len(b.buf) - b.rwDelta
}

func (b *circularBuffer) writeAvailable() int {
	return b.rwDelta
}

// copyUpTo reads at most n bytes into w. A short read at EOF is success.
func copyUpTo(w io.Writer, r io.Reader, n int64) (int64, error) {
	if n <= 0 {
		return 0, nil
	}
	written, err := io.CopyN(w, r, n)
	if err == nil {
		return written, nil
	}
	if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
		return written, nil
	}
	return written, err
}

// each section is supposed to hold SECTION_SIZE bytes at all times
type section struct {
	start     int64
	end       int64
	buf       *circularBuffer
	lastRead  time.Time
	lastWrite time.Time
}

func newSection(start int64, reader io.Reader) (*section, error) {
	buf := newCircularBuffer()
	n, err := copyUpTo(buf, reader, int64(buf.writeAvailable()))
	if err != nil {
		return nil, err
	}

	return &section{
		start:     start,
		end:       start + n,
		buf:       buf,
		lastRead:  time.Now(),
		lastWrite: time.Now(),
	}, nil
}

// the job of this function is to control the underlying
// buffer, allowing only valid reads (under SECTION_SIZE, etc)
func (s *section) get(amount int) ([]byte, int) {
	// how much CAN we read? cap that amount
	readAvailable := s.buf.readAvailable()
	amount = min(readAvailable, amount)
	s.lastRead = time.Now()
	return s.buf.get(amount), amount
}

func (s *section) write(reader io.Reader) error {
	// writes bytes to underlying circular buffer
	n, err := copyUpTo(s.buf, reader, int64(s.buf.writeAvailable()))
	if err != nil {
		return err
	}
	s.start += n
	s.end += n
	s.lastWrite = time.Now()
	return nil
}

// as i'm using VLC for streaming and not my own frontend yet
// i'll just store some set amount of bytes going forward from the last request
type LookAheadCache struct {
	sections map[string][]*section
}

func NewLookAheadCache() *LookAheadCache {
	return &LookAheadCache{make(map[string][]*section, 0)}
}

func (c *LookAheadCache) NewSection(name string, media MediaFile, fs fs.FS, start int64) error {
	// ensure slice exists
	if _, ok := c.sections[name]; !ok {
		c.sections[name] = []*section{}
	}

	// open media and write to section
	rs, err := OpenSeekMedia(fs, media, start)
	if err != nil {
		return err
	}
	defer rs.Close()

	// create the new section
	newSec, err := newSection(start, rs)
	if err != nil {
		return err
	}
	c.sections[name] = append(c.sections[name], newSec)

	return nil
}

func (c *LookAheadCache) FillNext(name string, media MediaFile, fs fs.FS, start int64) error {
	sections, ok := c.sections[name]
	if !ok {
		return errors.New("section not found")
	}
	var sec *section
	for _, s := range sections {
		if s.start != start {
			continue
		}
		sec = s
		break
	}
	if sec == nil {
		return errors.New("section not found")
	}

	// open media at the end of the cached range and append into the ring
	rs, err := OpenSeekMedia(fs, media, sec.end)
	if err != nil {
		return err
	}
	defer rs.Close()

	// Write into section
	if err = sec.write(rs); err != nil {
		return fmt.Errorf("failed to write into section: %w", err)
	}

	return nil
}

func (c *LookAheadCache) Read(name string, start int64, count int) ([]byte, int, bool) {
	sections, ok := c.sections[name]
	if !ok {
		return nil, 0, false
	}
	for _, s := range sections {
		// starting simple, starts must align
		// hopefully, they should mostly align, as we will cache for specific user requests
		// and their requests should come for the next byte
		if s.start != start {
			continue
		}
		data, n := s.get(count)
		return data, n, true
	}

	return nil, 0, false
}
