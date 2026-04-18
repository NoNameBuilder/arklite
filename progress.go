package main

import (
	"fmt"
	"io"
	"os"
	"sync/atomic"
	"time"
)

type byteCounter struct {
	n atomic.Int64
}

func (b *byteCounter) Add(v int64) { b.n.Add(v) }
func (b *byteCounter) Load() int64 { return b.n.Load() }

type countingReader struct {
	r io.Reader
	c *byteCounter
}

func (cr *countingReader) Read(p []byte) (int, error) {
	n, err := cr.r.Read(p)
	if n > 0 {
		cr.c.Add(int64(n))
	}
	return n, err
}

type progressPrinter struct {
	label string
	total int64
	done  *byteCounter
	stop  chan struct{}
}

func startProgress(label string, total int64, done *byteCounter) *progressPrinter {
	p := &progressPrinter{
		label: label,
		total: total,
		done:  done,
		stop:  make(chan struct{}),
	}
	go p.loop()
	return p
}

func (p *progressPrinter) loop() {
	tk := time.NewTicker(120 * time.Millisecond)
	defer tk.Stop()
	for {
		select {
		case <-p.stop:
			p.print(true)
			return
		case <-tk.C:
			p.print(false)
		}
	}
}

func (p *progressPrinter) print(done bool) {
	val := p.done.Load()
	if p.total > 0 {
		percent := float64(val) / float64(p.total) * 100.0
		if percent > 100 {
			percent = 100
		}
		fmt.Fprintf(os.Stderr, "\r%s %6.2f%% (%s/%s)", p.label, percent, humanBytes(val), humanBytes(p.total))
	} else {
		fmt.Fprintf(os.Stderr, "\r%s %s", p.label, humanBytes(val))
	}
	if done {
		fmt.Fprintln(os.Stderr)
	}
}

func (p *progressPrinter) Finish() {
	close(p.stop)
}

func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%dB", n)
	}
	div, exp := int64(unit), 0
	for q := n / unit; q >= unit; q /= unit {
		div *= unit
		exp++
	}
	pre := "KMGTPE"[exp]
	return fmt.Sprintf("%.1f%ciB", float64(n)/float64(div), pre)
}
