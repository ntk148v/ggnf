package main

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/google/go-github/github"
	"github.com/schollz/progressbar/v3"
)

var helpText = strings.TrimSpace(`
ggnf is Nerd Font downloader written in Golang.
https://github.com/ntk148v/ggnf

Usage:
  ggnf list                        - List all fonts
  ggnf download <font1> <font2>... - Download the given fonts
`)

type Font struct {
	Name             string `json:"name"`
	DownloadURL      string `json:"download_url"`
	InstalledVersion string `json:"installed"`
	LatestVersion    string `json:"latest"`
}

func main() {
	configDir, _ := os.UserConfigDir()
	dataFile := filepath.Join(configDir, "ggnf.json")

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Load data
	fonts, err := loadData(dataFile)
	if err != nil {
		log.Fatalln("Unable to load data from file due to: ", err)
	}

	defer saveData(dataFile, fonts)

	// Get Nerd Fonts latest release from Github
	if err := getLatestRelease(ctx, fonts); err != nil {
		log.Fatalln("Unable to get latest Nerd Fonts release due to: ", err)
	}

	args := os.Args[1:]
	if len(args) > 0 {
		switch args[0] {
		case "list":
			printJSON(fonts)
		case "download":
			var wg sync.WaitGroup
			for _, a := range args[1:] {
				wg.Add(1)
				go func(font string) {
					defer wg.Done()
					f, ok := fonts[font]
					if !ok {
						log.Printf("Unable to find font %s, make sure you enter the correct font\n", font)
						return
					}
					if f.InstalledVersion == f.LatestVersion {
						log.Printf("Font %s already installed, skip...", font)
						return
					}

					log.Printf("Downloading font %s ... It may take a while\n", font)
					if err := downloadFont(fonts[font]); err != nil {
						log.Fatalf("Unable to download font %s due to: %s", font, err)
					}

					// Update installed version
					f.InstalledVersion = f.LatestVersion
					fonts[font] = f

					log.Printf("Installing font %s ...\n", font)
				}(a)
			}
			wg.Wait()
			// Run fc-cache to update font list (Linux). Don't know how it works in Darwin, Windows
			cmd := exec.Command("fc-cache", "-f")
			if err := cmd.Run(); err != nil {
				log.Println("Error when running fc-cache:", err)
			}
		case "-h", "--help":
			fmt.Println(helpText)
		}
	} else {
		fmt.Println(helpText)
	}
}

// printJSON prints v as JSON encoded with indent to stdout. It panics on any error.
func printJSON(v interface{}) {
	w := json.NewEncoder(os.Stdout)
	w.SetIndent("", "\t")
	err := w.Encode(v)
	if err != nil {
		panic(err)
	}
}

// getLatestRelease fetches Github for the latest Nerd Fonts release
func getLatestRelease(ctx context.Context, fonts map[string]Font) error {
	// Github client
	client := github.NewClient(nil)
	latestRelease, _, err := client.Repositories.GetLatestRelease(ctx, "ryanoasis", "nerd-fonts")
	if err != nil {
		return err
	}

	// Randomly get the first font to check version
	if latestRelease.GetName() == fonts["3270"].LatestVersion {
		// Skip cause this is the already latest release
		return nil
	}

	log.Println("Found new release:", latestRelease.GetName())
	for _, a := range latestRelease.Assets {
		f := Font{
			Name:          strings.TrimSuffix(a.GetName(), ".zip"),
			LatestVersion: latestRelease.GetName(),
			DownloadURL:   a.GetBrowserDownloadURL(),
		}
		if tmp, ok := fonts[f.Name]; ok {
			f.InstalledVersion = tmp.InstalledVersion
		}

		fonts[f.Name] = f
	}

	return nil
}

// isRoot checks whether the current user is root or not
func isRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		log.Fatalln("Unable to get current user:", err)
	}
	return currentUser.Username == "root"
}

// getFontDir gets font directory
func getFontDir() string {
	var dir string

	switch runtime.GOOS {
	case "windows":

	case "darwin", "ios":

	case "plan9":

	default: // Unix
		if isRoot() {
			dir = "/usr/local/share/fonts"
		} else {
			home, _ := os.UserHomeDir()
			dir = filepath.Join(home, ".local/share/fonts")
		}
	}

	// Create font dir
	dir = filepath.Join(dir, "NerdFonts")
	os.MkdirAll(dir, os.ModePerm)
	return dir
}

// downloadFont gets the font from Github release and extract
// to the right place
func downloadFont(font Font) error {
	archivePath := filepath.Join(os.TempDir(), font.Name+".zip")
	// Create the file
	out, err := os.Create(archivePath)
	if err != nil {
		return err
	}

	resp, err := http.Get(font.DownloadURL)
	if err != nil {
		return err
	}

	defer func() {
		resp.Body.Close()
		out.Close()
		os.Remove(archivePath)
	}()

	bar := progressbar.DefaultBytes(
		resp.ContentLength,
		"downloading",
	)

	// Writer the body to file
	_, err = io.Copy(io.MultiWriter(out, bar), resp.Body)
	if err != nil {
		return err
	}

	// Unzip
	return unzip(archivePath, filepath.Join(getFontDir(), font.Name))
}

// loadData gets the list of fonts
func loadData(dataFile string) (map[string]Font, error) {
	fonts := make(map[string]Font, 0)
	if _, err := os.Stat(dataFile); err != nil {
		if os.IsNotExist(err) {
			os.Create(dataFile)
			return fonts, nil
		}
		return fonts, err
	}

	raw, err := os.ReadFile(dataFile)
	if err != nil {
		return fonts, err
	}

	_ = json.Unmarshal(raw, &fonts)
	return fonts, nil
}

// saveData dumps the list of fonts to disk
func saveData(dataFile string, fonts map[string]Font) error {
	raw, err := json.MarshalIndent(fonts, "", "	")
	if err != nil {
		return err
	}

	return os.WriteFile(dataFile, raw, 0644)
}

// unzip - get from https://stackoverflow.com/questions/20357223/easy-way-to-unzip-file-with-golang
func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer func() {
		if err := r.Close(); err != nil {
			panic(err)
		}
	}()

	os.MkdirAll(dest, 0755)

	// Closure to address file descriptors issue with all the deferred .Close() methods
	extractAndWriteFile := func(f *zip.File) error {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer func() {
			if err := rc.Close(); err != nil {
				panic(err)
			}
		}()

		path := filepath.Join(dest, f.Name)

		// Check for ZipSlip (Directory traversal)
		if !strings.HasPrefix(path, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", path)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			os.MkdirAll(filepath.Dir(path), f.Mode())
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer func() {
				if err := f.Close(); err != nil {
					panic(err)
				}
			}()

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
		return nil
	}

	for _, f := range r.File {
		err := extractAndWriteFile(f)
		if err != nil {
			return err
		}
	}

	return nil
}
