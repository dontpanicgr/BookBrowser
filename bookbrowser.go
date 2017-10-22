package main

import (
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/geek1011/BookBrowser/modules/server"

	_ "github.com/geek1011/BookBrowser/formats/all"
	"github.com/geek1011/BookBrowser/modules/sigusr"
	"github.com/geek1011/BookBrowser/modules/util"
	"github.com/urfave/cli"
)

var curversion = "dev"

func main() {
	workdir, err := os.Getwd()
	if err != nil {
		log.Fatalf("Fatal error: %s\n", err)
	}

	deftempdir, err := ioutil.TempDir("", "bookbrowser")
	if err != nil {
		deftempdir = filepath.Join(workdir, "_temp")
	}

	app := cli.NewApp()
	app.Name = "BookBrowser"
	app.Usage = "Web-based eBook server supporting ePub and PDF."
	app.Version = curversion
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "bookdir, b",
			Value: workdir,
			Usage: "Load books from `DIR`. The directory must exist.",
		},
		cli.StringFlag{
			Name:  "tempdir, t",
			Value: deftempdir,
			Usage: "Use `DIR` as the location for storing temporary files such as cover thumbnails. The directory is created on start and deleted on exit, unless it already exists.",
		},
		cli.StringFlag{
			Name:  "addr, a",
			Value: ":8090",
			Usage: "`ADDR` is the address to bind the server to. It is in the format IP:PORT. The IP is optional.",
		},
		cli.BoolFlag{
			Name:  "nocovers, n",
			Usage: "Do not index covers",
		},
	}
	app.HideHelp = true
	app.Action = func(c *cli.Context) {
		bookdir := c.String("bookdir")
		tempdir := c.String("tempdir")
		nocovers := c.Bool("nocovers")
		noRemoveTempDir := false

		addr := c.String("addr")

		log.Printf("BookBrowser %s\n", curversion)

		if _, err := os.Stat(bookdir); err != nil {
			if os.IsNotExist(err) {
				log.Fatalf("Fatal error: book directory %s does not exist\n", bookdir)
			}
		}

		if fi, err := os.Stat(tempdir); err == nil || fi.IsDir() {
			noRemoveTempDir = true
			if tempdir == deftempdir {
				noRemoveTempDir = false
			}
		}

		bookdir, err = filepath.Abs(bookdir)
		if err != nil {
			log.Fatalf("Fatal error: Could not resolve book directory %s: %s\n", bookdir, err)
		}

		if _, err := os.Stat(tempdir); os.IsNotExist(err) {
			os.Mkdir(tempdir, os.ModePerm)
		}

		tempdir, err = filepath.Abs(tempdir)
		if err != nil {
			log.Fatalf("Fatal error: Could not resolve temp directory %s: %s\n", tempdir, err)
		}

		sigs := make(chan os.Signal, 1)
		signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
		go func() {
			<-sigs
			if noRemoveTempDir {
				log.Println("Not removing temp dir because dir already existed at start")
			} else {
				log.Println("Cleaning up temp dir")
				os.RemoveAll(tempdir)
			}
			os.Exit(0)
		}()

		if !strings.Contains(addr, ":") {
			log.Fatalln("Invalid listening address")
		}

		sp := strings.SplitN(addr, ":", 2)
		if sp[0] == "" {
			ip := util.GetIP()
			if ip != nil {
				log.Printf("This server can be accessed at http://%s:%s\n", ip.String(), sp[1])
			}
		}

		s := server.NewServer(addr, bookdir, tempdir, curversion, true, nocovers)
		s.RefreshBookIndex()

		if len(*s.Books) == 0 {
			log.Fatalln("Fatal error: no books found")
		}

		sigusr.Handle(func() {
			log.Println("Booklist refresh triggered by SIGUSR1")
			s.RefreshBookIndex()
		})

		err = s.Serve()
		if err != nil {
			log.Fatalf("Error starting server: %s\n", err)
		}
	}
	app.Run(os.Args)
}
