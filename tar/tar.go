package tar

import (
  "io"
)

type Header struct {

}

type Reader struct {
  inner io.Reader
}

func NewReader(r io.Reader) *Reader {
  return &Reader{}
}

func (r *Reader) Read(b []byte) (int, error) {
  return 0, nil
}

func (r *Reader) Next() (*Header, error) {
  return nil, nil
}

type Writer struct {
  inner io.Writer
}

func NewWriter(w io.Writer) *Writer {
  return &Writer{}
}

func (w *Writer) Write(b []byte) (int, error) {
  return 0, nil
}

func (w *Writer) Flush() error {
  return nil
}

func (w *Writer) Close() error {
  return nil
}
