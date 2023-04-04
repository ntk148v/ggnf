package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/google/go-github/github"
)

var (
	cacheFile string
	items     []list.Item
	docStyle  = lipgloss.NewStyle().Margin(1, 2)
)

type item struct {
	name      string
	url       string
	latest    string
	installed string
}

func (i item) Title() string { return i.name }
func (i item) Description() string {
	return fmt.Sprintf("Installed version: %s - Latest version: %s", i.installed, i.latest)
}
func (i item) LatestVersion() string  { return i.latest }
func (i item) CurrentVersion() string { return i.installed }
func (i item) DownloadURL() string    { return i.url }
func (i item) FilterValue() string    { return i.name }

type model struct {
	list list.Model
}

func (m model) Init() tea.Cmd {
	return nil
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		h, v := docStyle.GetFrameSize()
		m.list.SetSize(msg.Width-h, msg.Height-v)
	}

	var cmd tea.Cmd
	m.list, cmd = m.list.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return docStyle.Render(m.list.View())
}

func main() {
	err := createCacheFile()
	if err != nil {
		panic(err)
	}

	// Create context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Get the latest release Nerd Fonts
	if err := getLatestRelease(ctx); err != nil {
		panic(err)
	}

	m := model{list: list.New(items, list.NewDefaultDelegate(), 0, 0)}
	m.list.Title = "GGNF"

	p := tea.NewProgram(m, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}

// createCacheFile
func createCacheFile() error {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return err
	}

	cacheFile = filepath.Join(cacheDir, "ggnf.json")
	if _, err := os.Stat(cacheFile); err != nil {
		if os.IsNotExist(err) {
			os.Create(cacheFile)
			return setCache(map[string]string{})
		}
		return err
	}

	return nil
}

// getCache loads cache data from cache file
func getCache() (map[string]string, error) {
	var installed map[string]string
	raw, err := ioutil.ReadFile(cacheFile)
	if err != nil {
		return installed, nil
	}

	err = json.Unmarshal(raw, &installed)
	return installed, err
}

// setCache saves the input to cache file
func setCache(installed map[string]string) error {
	raw, err := json.Marshal(installed)
	if err != nil {
		return err
	}

	return ioutil.WriteFile(cacheFile, raw, 0644)
}

// getLatestRelease fetches Github for the latest Nerd Fonts release
func getLatestRelease(ctx context.Context) error {
	// Load installed fonts
	installed, err := getCache()
	if err != nil {
		return err
	}

	// Github client
	client := github.NewClient(nil)
	latestRelease, _, err := client.Repositories.GetLatestRelease(ctx, "ryanoasis", "nerd-fonts")
	if err != nil {
		return err
	}

	for _, a := range latestRelease.Assets {
		i := item{
			name:      strings.Trim(*a.Name, ".zip"),
			url:       *a.BrowserDownloadURL,
			latest:    *latestRelease.Name,
			installed: "None",
		}

		if v, ok := installed[i.name]; ok {
			i.installed = v
		}

		items = append(items, i)
	}
	return nil
}
