package logging

import (
	"io"
	"log"
	"strings"
	"time"
)

// stdlogWriter tees every Write to the original destination and the ring buffer.
type stdlogWriter struct {
	orig io.Writer
}

func (w *stdlogWriter) Write(p []byte) (int, error) {
	n, err := w.orig.Write(p)

	line := strings.TrimRight(string(p), "\n")
	msg := stripStdlogPrefix(line)
	addToRing(LogEntry{Time: time.Now(), Level: "info", Caller: "stdlib", Message: msg})
	if shouldLogToFile("info") {
		writeToFile(logLine("info", "stdlib", "%s", msg))
	}

	return n, err
}

// stripStdlogPrefix removes the standard log date/time prefix "YYYY/MM/DD HH:MM:SS ".
func stripStdlogPrefix(line string) string {
	// default format: "2006/01/02 15:04:05 " = 20 chars
	if len(line) > 20 && line[4] == '/' && line[7] == '/' && line[13] == ':' && line[16] == ':' {
		return line[20:]
	}
	return line
}

// InitInterceptor redirects the standard logger output through the ring buffer
// while keeping the original destination intact. Call once after Init().
func InitInterceptor() {
	orig := log.Writer()
	if _, already := orig.(*stdlogWriter); already {
		return
	}
	log.SetOutput(&stdlogWriter{orig: orig})
}
