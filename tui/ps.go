package tui

import (
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var helpStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type model struct {
	table   table.Model
	rows    []table.Row
	spinner spinner.Model
	loading bool
}

type DockerPS struct {
	ContainerID string
	Image       string
	Command     string
	Created     string
	Status      string
	Ports       string
	Names       string
}

// Run 'docker ps' and get the output
func getDockerPSOutput() (string, error) {
	cmd := exec.Command("docker", "ps", "--all", "--format", "{{.ID}}\t{{.Image}}\t{{.Command}}\t{{.CreatedAt}}\t{{.Status}}\t{{.Ports}}\t{{.Names}}")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// Parse the output of the command
func parseDockerPSOutput(output string) []DockerPS {
	var dockerPSList []DockerPS
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		dockerPS := DockerPS{
			ContainerID: strings.TrimSpace(fields[0]),
			Image:       strings.TrimSpace(fields[1]),
			Command:     strings.TrimSpace(fields[2]),
			Created:     strings.TrimSpace(fields[3]),
			Status:      strings.TrimSpace(fields[4]),
			Ports:       strings.TrimSpace(fields[5]),
			Names:       strings.TrimSpace(fields[6]),
		}
		dockerPSList = append(dockerPSList, dockerPS)
	}
	return dockerPSList
}

// Convert parsed data to table rows
func dockerPSToTableRows(dockerPSList []DockerPS) []table.Row {
	var rows []table.Row
	for _, dockerPS := range dockerPSList {
		row := table.Row{
			dockerPS.ContainerID,
			dockerPS.Image,
			dockerPS.Command,
			dockerPS.Created,
			dockerPS.Status,
			dockerPS.Ports,
			dockerPS.Names,
		}
		rows = append(rows, row)
	}
	return rows
}

// Extract the first host port from the port mapping string
func extractPort(portMapping string) (string, error) {
	re := regexp.MustCompile(`(\d+)->\d+/tcp`)
	matches := re.FindStringSubmatch(portMapping)
	if len(matches) < 2 {
		return "", fmt.Errorf("no port found in %s", portMapping)
	}
	return matches[1], nil
}

func initialModel() model {
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	return model{
		spinner: s,
		loading: true, // Start with the loading state
	}
}

func (m model) Init() tea.Cmd {
	return tea.Batch(m.spinner.Tick, loadDockerData)
}

func loadDockerData() tea.Msg {
	output, err := getDockerPSOutput()
	if err != nil {
		return errMsg{err}
	}
	return dockerDataMsg(output)
}

type errMsg struct{ err error }
type dockerDataMsg string

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc":
			if m.table.Focused() {
				m.table.Blur()
			} else {
				m.table.Focus()
			}
		case "q", "ctrl+c":
			return m, tea.Quit
		case "e":
			if len(m.table.SelectedRow()) > 0 {
				selectedID := strings.TrimSpace(m.table.SelectedRow()[0])
				status := m.table.SelectedRow()[4] // Get the status of the selected container
				if strings.Contains(status, "Exited") || strings.Contains(status, "Stopped") {
					fmt.Println("Cannot execute a stopped container.")
					return m, nil
				}
				return m, func() tea.Msg {
					// Suspend the program to allow the command to take over the terminal
					_ = syscall.Exec("/usr/bin/docker", []string{"docker", "exec", "-it", selectedID, "bash"}, os.Environ())
					return nil
				}
			}
		case "s":
			if len(m.table.SelectedRow()) > 0 {
				selectedID := strings.TrimSpace(m.table.SelectedRow()[0])
				return m, func() tea.Msg {
					c := exec.Command("docker", "stop", selectedID)
					if output, err := c.CombinedOutput(); err != nil {
						fmt.Printf("Error stopping container: %s\nOutput: %s\n", err, string(output))
					}
					return refreshMsg{}
				}
			}
		case "r":
			if len(m.table.SelectedRow()) > 0 {
				selectedID := strings.TrimSpace(m.table.SelectedRow()[0])
				return m, func() tea.Msg {
					c := exec.Command("docker", "restart", selectedID)
					if output, err := c.CombinedOutput(); err != nil {
						fmt.Printf("Error restarting container: %s\nOutput: %s\n", err, string(output))
					}
					return refreshMsg{}
				}
			}
		case "d":
			if len(m.table.SelectedRow()) > 0 {
				selectID := strings.TrimSpace(m.table.SelectedRow()[0])
				return m, func() tea.Msg {
					c := exec.Command("docker", "rm", "--volumes", selectID)
					if output, err := c.CombinedOutput(); err != nil {
						fmt.Println("Error deleting container: ", string(output))
					}
					return refreshMsg{}
				}
			}
		case "o":
			if len(m.table.SelectedRow()) > 0 {
				portMapping := m.table.SelectedRow()[5]
				port, err := extractPort(portMapping)
				if err != nil {
					fmt.Printf("Error extracting port: %s\n", err)
					return m, nil
				}
				url := fmt.Sprintf("http://localhost:%s", port)
				exec.Command("xdg-open", url).Start()
				return m, nil
			}
		}
	case dockerDataMsg:
		dockerPSList := parseDockerPSOutput(string(msg))
		rows := dockerPSToTableRows(dockerPSList)
		m.rows = rows
		m.loading = false // Data has been loaded
		m.table.SetRows(rows)

	case errMsg:
		m.loading = false
		fmt.Printf("Error loading data: %s\n", msg.err)

	case refreshMsg:
		m.refreshTableData()
	}
	m.table, cmd = m.table.Update(msg)
	m.spinner, cmd = m.spinner.Update(msg)
	return m, cmd
}

type refreshMsg struct{}

func (m *model) refreshTableData() {
	output, err := getDockerPSOutput()
	if err != nil {
		fmt.Println("Error running docker ps:", err)
		return
	}

	dockerPSList := parseDockerPSOutput(output)
	m.rows = dockerPSToTableRows(dockerPSList)
	m.table.SetRows(m.rows)
}

func (m model) View() string {
	if m.loading {
		return fmt.Sprintf("\n\n %s Loading...", m.spinner.View())
	}
	styledRows := make([]table.Row, len(m.rows))
	// Define the style for stopped containers
	grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
	for i, row := range m.rows {
		// Check the status
		if strings.Contains(row[4], "Exited") || strings.Contains(row[4], "Stopped") {
			styledRow := make(table.Row, len(row))
			for j, cell := range row {
				// Apply grey style to each cell in the row
				styledRow[j] = grayStyle.Render(cell)
			}
			styledRows[i] = styledRow
		} else {
			// Keep original style if not stopped
			styledRows[i] = row
		}
	}

	// Update the table rows with the styled rows for rendering
	m.table.SetRows(styledRows)
	return baseStyle.Render(m.table.View()) + "\n" + m.helpView()
}

func (m model) helpView() string {
	return helpStyle("\n  ↑/↓: Navigate • q: Quit • s: Stop • r: Restart/Start • d: Delete • o: Open in browser\n")
}

func main() {
	m := initialModel()

	columns := []table.Column{
		{Title: "Container ID", Width: 20},
		{Title: "Image", Width: 20},
		{Title: "Command", Width: 20},
		{Title: "Created", Width: 20},
		{Title: "Status", Width: 20},
		{Title: "Ports", Width: 20},
		{Title: "Names", Width: 15},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)
	s.Selected = s.Selected.
		Foreground(lipgloss.Color("229")).
		Background(lipgloss.Color("57")).
		Bold(true)

	m.table = t
	m.table.SetStyles(s)

	if _, err := tea.NewProgram(&m, tea.WithAltScreen()).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
