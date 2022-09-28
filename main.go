/*
TunnelEffect (c) 2022 by Mikhail Kondrashin (mkondrashin@gmail.com)

main.go

Monitor unitily used to profile OS to check what file system operations
generate what github.com/rjeczalik/notify package events. Used to tune monitor_dispatch.go.

Results: See monitor_dispatch.go
*/

package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/rjeczalik/notify"
)

const pause = 10 * time.Millisecond

type action interface {
	Setup()
	Action()
	Name() string
}

type baseAction struct {
	root string
}

func (b *baseAction) Setup() {}

func (b *baseAction) Join(path string) string {
	return filepath.Join(b.root, path)
}

func (b *baseAction) Create(fileName string) *os.File {
	f, err := os.Create(b.Join(fileName))
	if err != nil {
		panic(err)
	}
	return f
}

type empty struct {
	baseAction
}

func (a *empty) Name() string {
	return "empty"
}

func (a *empty) Action() {
	f := a.Create(a.Name())
	f.Close()
}

type oneByte struct {
	baseAction
}

func (a *oneByte) Name() string {
	return "1byte"
}

func (a *oneByte) Action() {
	f := a.Create(a.Name())
	defer f.Close()
	_, err := f.Write([]byte{1})
	if err != nil {
		panic(err)
	}
}

type oneMegabyte struct {
	baseAction
}

func (a *oneMegabyte) Name() string {
	return "1M"
}
func (a *oneMegabyte) Action() {
	f := a.Create(a.Name())
	for i := 0; i < 1000; i++ {
		var buf [1000]byte
		_, err := f.Write(buf[:])
		if err != nil {
			panic(err)
		}
	}
	err := f.Close()
	if err != nil {
		panic(err)
	}
}

type oneAndOneMegabyte struct {
	baseAction
}

func (a *oneAndOneMegabyte) Name() string {
	return "1and1M"
}
func (a *oneAndOneMegabyte) Action() {
	f := a.Create(a.Name())
	for i := 0; i < 1024*1024; i++ {
		_, err := f.Write([]byte{1})
		if err != nil {
			panic(err)
		}
	}
	time.Sleep(pause)
	for i := 0; i < 1024*1024; i++ {
		_, err := f.Write([]byte{1})
		if err != nil {
			panic(err)
		}
	}
	f.Close()
}

type remove struct {
	baseAction
}

func (a *remove) Name() string {
	return "delete"
}

func (a *remove) Setup() {
	f := a.Create(a.Name())
	_, err := f.Write([]byte{1})
	if err != nil {
		panic(err)
	}
	err = f.Close()
	if err != nil {
		panic(err)
	}
}

func (a *remove) Action() {
	err := os.Remove(a.Join(a.Name()))
	if err != nil {
		panic(err)
	}
}

type moveOutside struct {
	baseAction
}

func (a *moveOutside) Name() string {
	return "move outside"
}

func (a *moveOutside) Setup() {
	f := a.Create(a.Name())
	_, err := f.Write([]byte{1})
	if err != nil {
		panic(err)
	}
	err = f.Close()
	if err != nil {
		panic(err)
	}
}

func (a *moveOutside) Action() {
	tgtFolder := filepath.Dir(a.root)
	tgtPath := filepath.Join(tgtFolder, a.Name())
	err := os.Rename(a.Join(a.Name()), tgtPath)
	if err != nil {
		panic(err)
	}
}

type moveAside struct {
	baseAction
}

func (a *moveAside) Name() string {
	return "move aside"
}

func (a *moveAside) Setup() {
	f := a.Create(a.Name())
	_, err := f.Write([]byte{1})
	if err != nil {
		panic(err)
	}
	err = f.Close()
	if err != nil {
		panic(err)
	}
}

func (a *moveAside) Action() {
	tgtPath := a.Join(a.Name() + " target")
	err := os.Rename(a.Join(a.Name()), tgtPath)
	if err != nil {
		panic(err)
	}
}

type moveFromOutside struct {
	baseAction
	sourcePath string
}

func (a *moveFromOutside) Name() string {
	return "move from outside"
}

func (a *moveFromOutside) Setup() {
	a.sourcePath = filepath.Join(filepath.Dir(a.root), a.Name())
	f, err := os.Create(a.sourcePath)
	if err != nil {
		panic(err)
	}
	_, err = f.Write([]byte{1})
	if err != nil {
		panic(err)
	}
	err = f.Close()
	if err != nil {
		panic(err)
	}
}

func (a *moveFromOutside) Action() {
	err := os.Rename(a.sourcePath, a.Join(a.Name()))
	if err != nil {
		panic(err)
	}
}

type generator struct {
	actions []action
}

func newGenerator(sourcePath string) *generator {
	actions := []action{
		&empty{baseAction{sourcePath}},
		&oneByte{baseAction{sourcePath}},
		&oneMegabyte{baseAction{sourcePath}},
		&oneAndOneMegabyte{baseAction{sourcePath}},
		&remove{baseAction{sourcePath}},
		&moveOutside{baseAction{sourcePath}},
		&moveAside{baseAction{sourcePath}},
		&moveFromOutside{baseAction{sourcePath}, ""},
	}
	return &generator{actions: actions}
}

func (g *generator) Setup() {
	for n, a := range g.actions {
		fmt.Printf("Setup %02d: %s\n", n, a.Name())
		a.Setup()
	}
}

func (g *generator) Action() {
	for n, a := range g.actions {
		fmt.Printf("Action %02d: %s\n", n, a.Name())
		a.Action()
	}
}

func main() {
	rootName := "testing_monitor"
	root, err := filepath.Abs(rootName)
	if err != nil {
		panic(err)
	}
	if err := os.RemoveAll(root); err != nil {
		panic(err)
	}
	if err := os.Mkdir(root, 0755); err != nil {
		panic(err)
	}
	source := "source"
	sourcePath := filepath.Join(root, source)
	if err := os.Mkdir(sourcePath, 0755); err != nil {
		panic(err)
	}
	g := newGenerator(sourcePath)
	g.Setup()
	logName := "monitor.log"
	logPath := filepath.Join(root, logName)
	if len(os.Args) == 2 {
		logPath = os.Args[1]
	}
	//	if os.Getenv("GITHUB_ACTION") == "" {
	logFile, err := os.Create(logPath)
	if err != nil {
		panic(err)
	}
	defer logFile.Close()
	//	}
	log := io.MultiWriter(logFile, os.Stdout)
	notifyChan := make(chan notify.EventInfo, 5)
	recursive := filepath.Join(sourcePath, "...")
	time.Sleep(pause)
	if err := notify.Watch(recursive, notifyChan, notify.All); err != nil {
		panic(err)
	}
	defer notify.Stop(notifyChan)
	//log.WriteString("Monitor Start\n")
	exit := make(chan struct{})

	operations := make(map[string][]string)

	go func() {
		//generateFiles(log, root, sourcePath)
		g.Action()
		time.Sleep(1 * time.Second)
		exit <- struct{}{}
	}()
	for {
		var event notify.EventInfo
		var ok bool
		select {
		case event, ok = <-notifyChan:
			if !ok {
				panic(fmt.Errorf("notify channel error"))
			}
		case <-exit:
			//log.WriteString("Monitor Stop\n")
			keys := make([]string, 0, len(operations))
			for k := range operations {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, a := range g.actions {
				k := a.Name()
				_, err := fmt.Fprintf(log, "%s: ", k)
				if err != nil {
					panic(err)
				}
				c := NewCompress(log)
				for _, o := range operations[k] {
					c.Write(o)
				}
				c.WriteCount()
				_, err = fmt.Fprintln(log)
				if err != nil {
					panic(err)
				}
			}
			return
		}
		path := filepath.Base(event.Path())
		//path := strings.TrimPrefix(event.Path(), root)
		//log.WriteString(fmt.Sprintf("File: %s, Event: %016b\n", path, event.Event()))
		var opName = map[notify.Event]string{
			notify.Write:  "W",
			notify.Create: "C",
			notify.Remove: "D",
			notify.Rename: "R",
		}
		op, ok := opName[event.Event()]
		if !ok {
			panic(fmt.Errorf("wrong event value: %016b", event.Event()))
		}
		operations[path] = append(operations[path], op)
		//log.WriteString(fmt.Sprintf("%s: %s\n", event.Event(), path))
		/*
			if event.Event()&notify.Write == notify.Write {
				log.WriteString(fmt.Sprintf("Write: %s: %s\n", event.Event(), path))
			}
			//log.Printf("Write:  %s: %s", event.Event(), event.Path())
			if event.Event()&notify.Create == notify.Create {
				log.WriteString(fmt.Sprintf("Write: %s: %s\n", event.Event(), path))
			}
			if event.Event()&notify.Remove == notify.Remove {
				log.WriteString(fmt.Sprintf("Remove: %s: %s\n", event.Event(), path))
			}
			if event.Event()&notify.Rename == notify.Rename {
				log.WriteString(fmt.Sprintf("Rename: %s: %s\n", event.Event(), path))
			}
		*/
	}
}

type Compress struct {
	last      string
	lastCount int
	wr        io.Writer
}

func NewCompress(wr io.Writer) *Compress {
	return &Compress{
		last:      "",
		lastCount: 1,
		wr:        wr,
	}
}

func (c *Compress) Write(s string) error {
	if s == c.last {
		c.lastCount++
		return nil
	}
	c.WriteCount()
	c.last = s
	_, err := c.wr.Write([]byte(s))
	return err
}

func (c *Compress) WriteCount() error {
	if c.lastCount == 1 {
		return nil
	}
	_, err := fmt.Fprintf(c.wr, "(%d)", c.lastCount)
	c.lastCount = 1
	return err
}
