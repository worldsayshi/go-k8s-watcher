// Package ui provides the TUI components for the resource viewer
package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/worldsayshi/go-k8s-watcher/pkg/db"
)

var (
	titleStyle        = lipgloss.NewStyle().MarginLeft(2)
	itemStyle         = lipgloss.NewStyle().PaddingLeft(4)
	selectedItemStyle = lipgloss.NewStyle().PaddingLeft(2).Foreground(lipgloss.Color("170"))
	paginationStyle   = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
	helpStyle         = list.DefaultStyles().HelpStyle.PaddingLeft(4).PaddingBottom(1)
	inputStyle        = lipgloss.NewStyle().Border(lipgloss.NormalBorder()).Padding(1).Width(80)
	appStyle          = lipgloss.NewStyle().Padding(1, 2, 0, 2)
)

// ResourceItem represents a Kubernetes resource in the list
type ResourceItem struct {
	resource db.Resource
}

// FilterValue returns the string to use for filtering
func (i ResourceItem) FilterValue() string {
	return fmt.Sprintf("%s %s %s", i.resource.Name, i.resource.Kind, i.resource.Namespace)
}

// Title returns the title of the item
func (i ResourceItem) Title() string {
	return fmt.Sprintf("%s/%s", i.resource.Kind, i.resource.Name)
}

// Description returns the description of the item
func (i ResourceItem) Description() string {
	ns := i.resource.Namespace
	if ns == "" {
		ns = "cluster-scoped"
	}
	return fmt.Sprintf("Namespace: %s, API Version: %s", ns, i.resource.APIVersion)
}

// ResourceUI is the main TUI application
type ResourceUI struct {
	list       list.Model
	input      textinput.Model
	db         *db.ResourceStore
	err        error
	resources  []db.Resource
	lastSearch string
	width      int
	height     int
}

// NewResourceUI creates a new TUI application
func NewResourceUI(store *db.ResourceStore) *ResourceUI {
	// Create text input field
	ti := textinput.New()
	ti.Placeholder = "Search resources..."
	ti.Focus()
	ti.Width = 80

	// Create the UI
	return &ResourceUI{
		list:  list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		input: ti,
		db:    store,
	}
}

// Init initializes the TUI application
func (r *ResourceUI) Init() tea.Cmd {
	return tea.Batch(
		textinput.Blink,
		r.performSearch(""),
	)
}

// performSearch executes the search and updates the list
func (r *ResourceUI) performSearch(query string) tea.Cmd {
	return func() tea.Msg {
		resources, err := r.db.Search(query)
		if err != nil {
			return errMsg{err}
		}
		return resourcesMsg{
			resources: resources,
			query:     query,
		}
	}
}

// resourcesMsg is a message containing search results
type resourcesMsg struct {
	resources []db.Resource
	query     string
}

// errMsg represents an error message
type errMsg struct {
	err error
}

// Update handles UI updates
func (r *ResourceUI) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyEsc:
			return r, tea.Quit
		case tea.KeyEnter:
			// Perform search when Enter is pressed
			r.lastSearch = r.input.Value()
			return r, r.performSearch(r.input.Value())
		}

	case tea.WindowSizeMsg:
		r.width = msg.Width
		r.height = msg.Height
		inputHeight := 3 // Height of input field with padding
		r.list.SetSize(msg.Width, msg.Height-inputHeight)

	case resourcesMsg:
		r.resources = msg.resources
		r.lastSearch = msg.query

		// Convert resources to list items
		var items []list.Item
		for _, resource := range r.resources {
			items = append(items, ResourceItem{resource: resource})
		}
		r.list.SetItems(items)

	case errMsg:
		r.err = msg.err
		return r, nil
	}

	var cmd tea.Cmd
	r.input, cmd = r.input.Update(msg)
	cmds = append(cmds, cmd)

	r.list, cmd = r.list.Update(msg)
	cmds = append(cmds, cmd)

	return r, tea.Batch(cmds...)
}

// View renders the TUI
func (r *ResourceUI) View() string {
	if r.err != nil {
		return fmt.Sprintf("Error: %v", r.err)
	}

	// Build the view
	var b strings.Builder
	b.WriteString(appStyle.Render(inputStyle.Render(r.input.View())))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("Found %d resources", len(r.resources)))
	if r.lastSearch != "" {
		b.WriteString(fmt.Sprintf(" matching '%s'", r.lastSearch))
	}
	b.WriteString("\n\n")
	b.WriteString(r.list.View())

	return b.String()
}

// Run starts the TUI application
func Run(store *db.ResourceStore) error {
	p := tea.NewProgram(NewResourceUI(store), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

// PeriodicRefresh sends refresh messages at regular intervals
func PeriodicRefresh(duration time.Duration) tea.Cmd {
	return tea.Tick(duration, func(time.Time) tea.Msg {
		return refreshMsg{}
	})
}

// refreshMsg is sent when it's time to refresh the data
type refreshMsg struct{}
