package main // import "github.com/rs/jplot"

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/rs/jplot/data"
	"github.com/rs/jplot/graph"
	"github.com/rs/jplot/osc"
)

func main() {
	url := flag.String("url", "", "URL to fetch every second. Read JSON objects from stdin if not specified.")
	interval := flag.Duration("interval", time.Second, "When url is provided, defines the interval between fetches."+
		" Note that counter fields are computed based on this interval.")
	steps := flag.Int("steps", 100, "Number of values to plot.")
	rows := flag.Int("rows", 0, "Limits the height of the graph output.")
	flag.Parse()

	if os.Getenv("TERM_PROGRAM") != "iTerm.app" {
		fatal("iTerm2 required")
	}
	if os.Getenv("TERM") == "screen" {
		fatal("screen and tmux not supported")
	}

	specs, err := data.ParseSpec(flag.Args())
	if err != nil {
		fatal("Cannot parse spec: ", err)
	}
	var dp *data.Points
	if *url != "" {
		dp = data.FromHTTP(*url, *interval, *steps)
	} else {
		dp = data.FromStdin(*steps)
	}
	defer dp.Close()
	dash := graph.Dash{
		Specs: specs,
		Data:  dp,
	}

	wg := &sync.WaitGroup{}
	wg.Add(1)
	defer wg.Wait()
	exit := make(chan struct{})
	defer close(exit)
	go func() {
		defer wg.Done()
		t := time.NewTicker(time.Second)
		defer t.Stop()
		c := make(chan os.Signal, 2)
		signal.Notify(c, os.Interrupt, syscall.SIGTERM)
		i := 0
		prepare(*rows)
		defer cleanup(*rows)
		for {
			select {
			case <-t.C:
				i++
				if i%120 == 0 {
					// Clear scrollback to avoid iTerm from eating all the memory.
					osc.ClearScrollback()
				}
				osc.CursorSavePosition()
				render(dash, *rows)
				osc.CursorRestorePosition()
			case <-exit:
				render(dash, *rows)
				return
			case <-c:
				cleanup(*rows)
				os.Exit(0)
			}
		}
	}()

	if err := dp.Run(specs); err != nil {
		fatal("Data source error: ", err)
	}
}

func fatal(a ...interface{}) {
	fmt.Println(append([]interface{}{"jplot: "}, a...)...)
	os.Exit(1)
}

func prepare(rows int) {
	osc.HideCursor()
	if rows == 0 {
		size, err := osc.Size()
		if err != nil {
			fatal("Cannot get window size: ", err)
		}
		rows = size.Row
	}
	print(strings.Repeat("\n", rows))
	osc.CursorMove(osc.Up, rows)
}

func cleanup(rows int) {
	osc.ShowCursor()
	if rows == 0 {
		size, _ := osc.Size()
		rows = size.Row
	}
	osc.CursorMove(osc.Down, rows)
	print("\n")
}

func render(dash graph.Dash, rows int) {
	size, err := osc.Size()
	if err != nil {
		fatal("Cannot get window size: ", err)
	}
	width, height := size.Width, size.Height
	if rows > 0 {
		height = size.Height / size.Row * rows
	} else {
		rows = size.Row
	}
	// Use iTerm2 image display feature.
	term := &osc.ImageWriter{}
	defer term.Close()
	dash.Render(term, width, height)
}
