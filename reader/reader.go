package reader

import (
	"io"
)

// Reader represents the source reader.
type Reader struct {
	// Using a very slightly modified version of
	// bytes.Buffer. Added a NextRune() method that
	// reads the next rune but doesn't move the cursor.
	Source *Buffer
}

// New initialises a reader.
func New(contents []byte) *Reader {
	return &Reader{Source: NewBuffer(contents)}
}

// Advance moves the cursor forward by one position.
func (r *Reader) Advance() (rune, error) {
	rn, _, err := r.Source.ReadRune()
	if err == io.EOF {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	return rn, nil
}

// Peek gets the next character but doesn't advance the cursor.
func (r *Reader) Peek() (rune, error) {
	rn, _, err := r.Source.NextRune()
	if err == io.EOF {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}

	return rn, nil
}

// Unread returns the cursor to the previously
// read character.
func (r *Reader) Unread() error {
	if err := r.Source.UnreadRune(); err != nil {
		return err
	}

	return nil
}
