package reader

import (
	"io"
)

// Reader.
type Reader struct {
	// Using a very slightly modified version of
	// bytes.Buffer. Added a NextRune() method that
	// reads the next rune but doesn't move the cursor.
	Source *Buffer
}

// Initialise a reader.
func New(contents []byte) *Reader {
	return &Reader{Source: NewBuffer(contents)}
}

// Advance the cursor by one position.
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

// Get the next character but don't advance the cursor.
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

// Unread the previosuly read character.
func (r *Reader) Unread() error {
	if err := r.Source.UnreadRune(); err != nil {
		return err
	}

	return nil
}
