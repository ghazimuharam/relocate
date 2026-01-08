package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/urfave/cli/v2"

	"github.com/ghazimuharam/relocate/internal/config"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

var (
	// Modern color palette
	primaryColor   = lipgloss.Color("#7DCFB6") // Mint green
	secondaryColor = lipgloss.Color("#E5E5E5") // Light gray
	accentColor    = lipgloss.Color("#FF6B6B") // Coral red
	dimColor       = lipgloss.Color("#6B7280") // Gray
	faintColor     = lipgloss.Color("#9CA3AF") // Light gray
	successColor   = lipgloss.Color("#10B981") // Green
	errorColor     = lipgloss.Color("#EF4444") // Red
	warningColor   = lipgloss.Color("#F59E0B") // Amber

	// Base styles (no fixed widths)
	baseStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E5E5E5"))

	// Section headers
	sectionHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(primaryColor).
				MarginBottom(1)

	// Detail labels
	detailLabelStyle = lipgloss.NewStyle().
				Foreground(dimColor).
				Width(12)

	// Detail values
	detailValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E5E5E5")).
				Bold(true)
)

// Pre-rendered indicators
var runningDot = lipgloss.NewStyle().Foreground(successColor).Render("●")
var stoppedDot = lipgloss.NewStyle().Foreground(dimColor).Render("●")

// Dynamic style builders based on terminal size
func (m model) titleBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(primaryColor).
		Background(lipgloss.Color("#1F2937")).
		Padding(0, 2).
		Width(m.width)
}

func (m model) headerStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(dimColor).
		Background(lipgloss.Color("#111827")).
		Padding(0, 2).
		Width(m.width)
}

func (m model) listContainerStyle() lipgloss.Style {
	listWidth := m.width / 2
	if listWidth < 30 {
		listWidth = 30
	}
	listHeight := m.height - 10
	if listHeight < 10 {
		listHeight = 10
	}

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(dimColor).
		PaddingRight(1).
		Width(listWidth).
		Height(listHeight)
}

func (m model) detailContainerStyle() lipgloss.Style {
	detailWidth := m.width - (m.width / 2)
	if detailWidth < 30 {
		detailWidth = 30
	}
	detailHeight := m.height - 10
	if detailHeight < 10 {
		detailHeight = 10
	}

	return lipgloss.NewStyle().
		Border(lipgloss.NormalBorder(), true, false, false, false).
		BorderForeground(dimColor).
		PaddingLeft(2).
		Width(detailWidth).
		Height(detailHeight)
}

func (m model) itemStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(lipgloss.Color("#E5E5E5")).
		Padding(0, 1)
}

func (m model) selectedItemStyle() lipgloss.Style {
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder(), true, true, true, true).
		BorderForeground(primaryColor).
		Bold(true)
}

func (m model) keySelectorStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(primaryColor).
		Padding(0, 2).
		MarginTop(1)
}

func (m model) statusBarStyle() lipgloss.Style {
	return lipgloss.NewStyle().
		Foreground(dimColor).
		Background(lipgloss.Color("#111827")).
		Padding(0, 2).
		Width(m.width)
}

func (m model) confirmStyle() lipgloss.Style {
	confirmWidth := 60
	if confirmWidth > m.width-4 {
		confirmWidth = m.width - 4
	}

	return lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(accentColor).
		Padding(1, 2).
		Align(lipgloss.Center).
		Width(confirmWidth)
}

// Global config, loaded on startup
var appConfig config.Config

// EC2Instance represents an EC2 instance
type EC2Instance struct {
	ID      string
	Name    string
	IP      string
	State   string
	Type    string
	Zone    string
	KeyName string
	AMI     string
}

// viewMode represents UI states
type viewMode int

const (
	viewNormal viewMode = iota
	viewConfirm
)

// Model for BubbleTea
type model struct {
	instances   []EC2Instance
	filtered    []EC2Instance
	cursor      int
	selected    bool
	loading     bool
	err         string
	profile     string
	region      string
	filterTag   string
	searchQuery string
	envMode     string // "staging" or "prod"
	mode        viewMode
	spinnerIdx  int
	lastUpdate  time.Time
	width       int // terminal width
	height      int // terminal height
}

// Messages
type instancesLoadedMsg struct {
	instances []EC2Instance
}

type errorMsg struct {
	err string
}

type tickMsg struct{}

// fuzzyMatch performs fuzzy matching - returns true if all characters in query
// appear in target in order, allowing non-matching characters in between.
// For example: "commerceapp" matches "commerce-app", "ca" matches "commerce-app"
func fuzzyMatch(query, target string) bool {
	if query == "" {
		return true
	}

	query = strings.ToLower(query)
	target = strings.ToLower(target)

	queryIdx := 0
	for _, targetChar := range target {
		if queryIdx < len(query) && rune(query[queryIdx]) == targetChar {
			queryIdx++
		}
	}

	return queryIdx == len(query)
}

func initialModel(profile, region, filterTag string) model {
	// Apply defaults from config if not provided
	if profile == "" && appConfig.Defaults.AWSProfile != "" {
		profile = appConfig.Defaults.AWSProfile
	}
	if region == "" && appConfig.Defaults.AWSRegion != "" {
		region = appConfig.Defaults.AWSRegion
	}

	return model{
		loading:    true,
		cursor:     0,
		profile:    profile,
		region:     region,
		filterTag:  filterTag,
		envMode:    "staging",
		mode:       viewNormal,
		spinnerIdx: 0,
		lastUpdate: time.Now(),
		width:      80,
		height:     24,
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		tea.EnterAltScreen,
		loadInstances(m.profile, m.region, m.filterTag),
		tick(),
	)
}

func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if m.mode == viewConfirm {
			if msg.String() == "y" || msg.String() == "Y" || msg.String() == " " {
				m.selected = true
				return m, tea.Quit
			}
			if msg.String() == "n" || msg.String() == "N" || msg.Type == tea.KeyEsc {
				m.mode = viewNormal
				return m, nil
			}
			return m, nil
		}

		switch msg.Type {
		case tea.KeyCtrlC:
			return m, tea.Quit

		case tea.KeyEsc:
			if m.searchQuery != "" {
				m.searchQuery = ""
				m.filterInstances()
				m.cursor = 0
			} else {
				return m, tea.Quit
			}

		case tea.KeyEnter:
			if m.loading || m.err != "" || len(m.filtered) == 0 {
				return m, nil
			}
			m.mode = viewConfirm

		case tea.KeyUp, tea.KeyDown:
			if msg.Type == tea.KeyUp && m.cursor > 0 {
				m.cursor--
			}
			if msg.Type == tea.KeyDown && m.cursor < len(m.filtered)-1 {
				m.cursor++
			}

		case tea.KeyTab:
			if m.envMode == "staging" {
				m.envMode = "prod"
			} else {
				m.envMode = "staging"
			}
			m.filterInstances()
			m.cursor = 0

		case tea.KeyBackspace:
			if len(m.searchQuery) > 0 {
				m.searchQuery = m.searchQuery[:len(m.searchQuery)-1]
				m.filterInstances()
				if m.cursor >= len(m.filtered) {
					m.cursor = max(0, len(m.filtered)-1)
				}
			}

		case tea.KeyRunes:
			switch msg.String() {
			case "k":
				if m.cursor > 0 {
					m.cursor--
				}
			case "j":
				if m.cursor < len(m.filtered)-1 {
					m.cursor++
				}
			case "1":
				if m.envMode != "staging" {
					m.envMode = "staging"
					m.filterInstances()
					m.cursor = 0
				}
			case "2":
				if m.envMode != "prod" {
					m.envMode = "prod"
					m.filterInstances()
					m.cursor = 0
				}
			default:
				m.searchQuery += msg.String()
				m.filterInstances()
				m.cursor = 0
			}
		}

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tickMsg:
		m.spinnerIdx = (m.spinnerIdx + 1) % 4
		if m.loading {
			return m, tick()
		}
		return m, nil

	case instancesLoadedMsg:
		m.instances = msg.instances
		m.filterInstances()
		m.loading = false
		return m, nil

	case errorMsg:
		m.loading = false
		m.err = msg.err
		return m, nil
	}

	return m, nil
}

func (m *model) filterInstances() {
	// First filter by environment
	var envFiltered []EC2Instance
	for _, inst := range m.instances {
		// Filter instances based on KeyName matching the environment
		if m.envMode == "staging" && strings.Contains(inst.KeyName, "staging") {
			envFiltered = append(envFiltered, inst)
		} else if m.envMode == "prod" && strings.Contains(inst.KeyName, "prod") {
			envFiltered = append(envFiltered, inst)
		}
	}

	// Then apply search query if present
	if m.searchQuery == "" {
		m.filtered = envFiltered
		return
	}

	m.filtered = nil
	for _, inst := range envFiltered {
		if fuzzyMatch(m.searchQuery, inst.Name) ||
			fuzzyMatch(m.searchQuery, inst.ID) ||
			fuzzyMatch(m.searchQuery, inst.IP) ||
			fuzzyMatch(m.searchQuery, inst.Type) {
			m.filtered = append(m.filtered, inst)
		}
	}
}

func (m model) View() string {
	if m.mode == viewConfirm {
		return m.renderMain() + "\n" + m.renderConfirm()
	}
	return m.renderMain()
}

func (m model) renderMain() string {
	var b strings.Builder

	// Title bar
	b.WriteString(m.titleBarStyle().Render(" relocate "))
	b.WriteString("\n")

	// Header bar
	var headerParts []string
	headerParts = append(headerParts, fmt.Sprintf("Profile: %s", m.profile))
	headerParts = append(headerParts, fmt.Sprintf("Region: %s", m.region))
	headerParts = append(headerParts, fmt.Sprintf("Instances: %d", len(m.filtered)))
	b.WriteString(m.headerStyle().Render(strings.Join(headerParts, "  •  ")))
	b.WriteString("\n\n")

	if m.loading {
		return b.String() + m.renderLoading()
	}

	if m.err != "" {
		return b.String() + m.renderError()
	}

	// Main content: list + details side by side
	content := lipgloss.JoinHorizontal(lipgloss.Top,
		m.renderList(),
		m.renderDetails(),
	)
	b.WriteString(content)
	b.WriteString("\n\n")

	// Key selector
	b.WriteString(m.renderKeySelector())
	b.WriteString("\n")

	// Status bar
	b.WriteString(m.statusBarStyle().Render(m.renderStatusBar()))

	return b.String()
}

func (m model) renderLoading() string {
	spinner := []string{"◜", "◠", "◝", "◞"}[m.spinnerIdx]
	loadingStyle := lipgloss.NewStyle().
		Foreground(primaryColor).
		Margin(1, 2)
	return loadingStyle.Render(spinner + " Loading instances...")
}

func (m model) renderError() string {
	errStyle := lipgloss.NewStyle().
		Foreground(errorColor).
		Bold(true).
		Margin(1, 2)
	return errStyle.Render("✕ " + m.err)
}

func (m model) renderList() string {
	var items []string

	// Header
	items = append(items, sectionHeaderStyle.Render("Instances"))

	// Calculate visible items based on container height
	visibleItems := m.height - 12
	if visibleItems < 5 {
		visibleItems = 5
	}

	// Calculate viewport bounds with proper scrolling
	start := 0
	end := min(len(m.filtered), visibleItems)

	// Scroll the view to keep cursor visible
	if m.cursor >= visibleItems {
		start = m.cursor - visibleItems + 1
		end = min(len(m.filtered), m.cursor+1)
	}

	for i := start; i < end; i++ {
		inst := m.filtered[i]
		stateIcon := runningDot
		if inst.State != "running" {
			stateIcon = stoppedDot
		}

		name := inst.Name
		if name == "" {
			name = inst.ID
		}

		// Truncate name based on available width
		maxNameLen := (m.width / 2) - 8
		if maxNameLen < 15 {
			maxNameLen = 15
		}
		if len(name) > maxNameLen {
			name = name[:maxNameLen-3] + "..."
		}

		item := fmt.Sprintf("%s %s", stateIcon, name)

		if i == m.cursor {
			items = append(items, m.selectedItemStyle().Render(item))
		} else {
			items = append(items, m.itemStyle().Render(item))
		}
	}

	// Fill remaining space
	totalLines := visibleItems + 1 // +1 for header
	for len(items) < totalLines {
		items = append(items, "")
	}

	return m.listContainerStyle().Render(lipgloss.JoinVertical(lipgloss.Left, items...))
}

func (m model) renderDetails() string {
	if len(m.filtered) == 0 {
		return m.detailContainerStyle().Render("")
	}

	inst := m.filtered[m.cursor]

	details := []string{
		sectionHeaderStyle.Render("Details"),
		"",
		detailLabelStyle.Render("Name") + " " + detailValueStyle.Render(inst.Name),
		"",
		detailLabelStyle.Render("ID") + " " + detailValueStyle.Render(inst.ID),
		"",
		detailLabelStyle.Render("AMI") + " " + detailValueStyle.Render(inst.AMI),
		"",
		detailLabelStyle.Render("IP") + " " + detailValueStyle.Render(inst.IP),
		"",
		detailLabelStyle.Render("Type") + " " + detailValueStyle.Render(inst.Type),
		"",
		detailLabelStyle.Render("Zone") + " " + detailValueStyle.Render(inst.Zone),
		"",
		detailLabelStyle.Render("State") + " " + detailValueStyle.Render(inst.State),
		"",
		detailLabelStyle.Render("Key") + " " + detailValueStyle.Render(inst.KeyName),
	}

	return m.detailContainerStyle().Render(lipgloss.JoinVertical(lipgloss.Left, details...))
}

func (m model) renderKeySelector() string {
	stagingStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#111827")).
		Background(lipgloss.Color("#9CA3AF")).
		Padding(0, 2)

	prodStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#111827")).
		Background(lipgloss.Color("#9CA3AF")).
		Padding(0, 2)

	activeStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#111827")).
		Background(primaryColor).
		Bold(true).
		Padding(0, 2)

	var stagingBtn, prodBtn string
	if m.envMode == "staging" {
		stagingBtn = activeStyle.Render(" [1] Staging ")
		prodBtn = prodStyle.Render(" [2] Prod ")
	} else {
		stagingBtn = stagingStyle.Render(" [1] Staging ")
		prodBtn = activeStyle.Render(" [2] Prod ")
	}

	return m.keySelectorStyle().Render(
		lipgloss.JoinHorizontal(lipgloss.Left, stagingBtn, prodBtn),
	)
}

func (m model) renderConfirm() string {
	if len(m.filtered) == 0 {
		return ""
	}

	inst := m.filtered[m.cursor]
	keyName, err := appConfig.GetSSHKey(m.envMode)
	if err != nil {
		keyName = "(not configured)"
	}

	content := lipgloss.JoinVertical(lipgloss.Center,
		lipgloss.NewStyle().Bold(true).Foreground(accentColor).Render("Connect to instance?"),
		"",
		detailLabelStyle.Render("Name")+detailValueStyle.Render(inst.Name),
		detailLabelStyle.Render("IP")+detailValueStyle.Render(inst.IP),
		detailLabelStyle.Render("Key")+detailValueStyle.Render(keyName),
		"",
		lipgloss.NewStyle().Foreground(dimColor).Render("[Y] Yes  [N] No  [ESC] Cancel"),
	)

	return m.confirmStyle().Render(content)
}

func (m model) renderStatusBar() string {
	var parts []string

	if m.searchQuery != "" {
		parts = append(parts, fmt.Sprintf("Search: %s", m.searchQuery))
	}

	parts = append(parts, "↑↓ navigate")
	parts = append(parts, "Enter connect")
	parts = append(parts, "[1/2] env")
	parts = append(parts, "type search")
	parts = append(parts, "Ctrl+C quit")

	return strings.Join(parts, "  •  ")
}

func loadInstances(profile, region, filterTag string) tea.Cmd {
	return func() tea.Msg {
		cfg, err := awsconfig.LoadDefaultConfig(context.TODO(),
			awsconfig.WithSharedConfigProfile(profile),
			awsconfig.WithRegion(region),
		)
		if err != nil {
			return errorMsg{err: "Failed to load AWS config"}
		}

		client := ec2.NewFromConfig(cfg)

		filters := []types.Filter{
			{
				Name:   aws.String("instance-state-name"),
				Values: []string{"running"},
			},
		}

		if filterTag != "" {
			parts := strings.SplitN(filterTag, "=", 2)
			if len(parts) == 2 {
				filters = append(filters, types.Filter{
					Name:   aws.String("tag:" + parts[0]),
					Values: []string{parts[1]},
				})
			}
		}

		resp, err := client.DescribeInstances(context.TODO(), &ec2.DescribeInstancesInput{
			Filters: filters,
		})
		if err != nil {
			return errorMsg{err: fmt.Sprintf("AWS error: %v", err)}
		}

		var instances []EC2Instance
		for _, res := range resp.Reservations {
			for _, inst := range res.Instances {
				name := ""
				for _, tag := range inst.Tags {
					if *tag.Key == "Name" {
						name = *tag.Value
						break
					}
				}

				ip := ""
				if inst.PublicIpAddress != nil {
					ip = *inst.PublicIpAddress
				} else if inst.PrivateIpAddress != nil {
					ip = *inst.PrivateIpAddress
				}

				zone := ""
				if inst.Placement != nil {
					zone = *inst.Placement.AvailabilityZone
				}

				keyName := ""
				if inst.KeyName != nil {
					keyName = *inst.KeyName
				}

				ami := ""
				if inst.ImageId != nil {
					ami = *inst.ImageId
				}

				instances = append(instances, EC2Instance{
					ID:      *inst.InstanceId,
					Name:    name,
					IP:      ip,
					State:   string(inst.State.Name),
					Type:    string(inst.InstanceType),
					Zone:    zone,
					KeyName: keyName,
					AMI:     ami,
				})
			}
		}

		// Sort instances alphabetically by name (or ID if name is empty)
		slices.SortFunc(instances, func(a, b EC2Instance) int {
			aName := a.Name
			if aName == "" {
				aName = a.ID
			}
			bName := b.Name
			if bName == "" {
				bName = b.ID
			}
			return strings.Compare(aName, bName)
		})

		return instancesLoadedMsg{instances: instances}
	}
}

func main() {
	// Load configuration on startup
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Config validation failed: %v\n", err)
		os.Exit(1)
	}
	appConfig = cfg

	app := &cli.App{
		Name:    "relocate",
		Version: fmt.Sprintf("%s (commit: %s, built at: %s)", version, commit, date),
		Usage:   "Quick SSH to AWS instances",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "profile",
				Aliases: []string{"p"},
				Usage:   "AWS profile",
				Value:   "default",
			},
			&cli.StringFlag{
				Name:    "region",
				Aliases: []string{"r"},
				Usage:   "AWS region",
				Value:   "ap-southeast-1",
			},
			&cli.StringFlag{
				Name:    "filter",
				Aliases: []string{"f"},
				Usage:   "Filter by tag (e.g., Environment=staging)",
			},
			&cli.StringFlag{
				Name:    "user",
				Aliases: []string{"u"},
				Usage:   "SSH user",
				Value:   "ubuntu",
			},
		},
		Action: func(ctx *cli.Context) error {
			p := tea.NewProgram(
				initialModel(ctx.String("profile"), ctx.String("region"), ctx.String("filter")),
				tea.WithAltScreen(),
			)

			finalModel, err := p.Run()
			if err != nil {
				return err
			}

			m := finalModel.(model)
			if m.selected && len(m.filtered) > 0 {
				inst := m.filtered[m.cursor]

				keyName, err := appConfig.GetSSHKey(m.envMode)
				if err != nil {
					return fmt.Errorf("SSH key not configured for %s: %w", m.envMode, err)
				}
				keyPath := filepath.Join(os.Getenv("HOME"), ".ssh", keyName)

				// Get SSH user from config or CLI flag
				sshUser := ctx.String("user")
				if sshUser == "" && appConfig.Defaults.SSHUser != "" {
					sshUser = appConfig.Defaults.SSHUser
				}
				if sshUser == "" {
					sshUser = "ubuntu"
				}

				fmt.Print("\033[H\033[2J")
				fmt.Printf("Connecting to %s (%s)...\n\n", inst.Name, inst.IP)

				cmd := exec.Command("ssh", "-i", keyPath, sshUser+"@"+inst.IP)
				cmd.Stdin = os.Stdin
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				return cmd.Run()
			}

			return nil
		},
	}

	if err := app.Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
