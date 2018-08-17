package main

import (
	"bytes"
	"io"
	"sync"
	"testing"
)

func TestLogLineWriter(t *testing.T) {

	buf := new(bytes.Buffer)

	var fakeLogWriter io.Writer
	fakeLogWriter = newSyncWriter(muxWriter{buf})

	content1 := "XXXXXXXXXXXXXX"
	content2 := "YYYYYYYYYYYYYY"
	content3 := "ZZZZZZZZZZZZZZ"

	contents := []string{
		content1, content2, content3,
	}

	wg := &sync.WaitGroup{}
	for _, content := range contents {
		w := newLineWriter(fakeLogWriter)
		content := content
		wg.Add(1)
		go func() {
			for i := 0; i < 99999; i++ {
				w.Write([]byte(content + "\n"))
			}
			wg.Done()
		}()
	}
	wg.Wait()

	finalContent := buf.String()
	uniqueLines := make(map[string]struct{})

	NewLogBuilderWithMaxBytes(0, finalContent).
		Trim("\n").
		Split("\n").
		Filter(func(line string) bool {
			uniqueLines[line] = struct{}{}
			return true
		}).Log()

	for line := range uniqueLines {
		if line != content1 && line != content2 && line != content3 {
			t.Errorf("malformed log: %s", line)
			t.Fail()
		}
	}

}
