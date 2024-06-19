package tui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var baseStyle = lipgloss.NewStyle().
	BorderStyle(lipgloss.NormalBorder()).
	BorderForeground(lipgloss.Color("240"))

type model struct {
	table table.Model
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
			ContainerID: fields[0],
			Image:       fields[1],
			Command:     fields[2],
			Created:     fields[3],
			Status:      fields[4],
			Ports:       fields[5],
			Names:       fields[6],
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

func (m model) Init() tea.Cmd { return nil }

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
		case "enter":
			if len(m.table.SelectedRow()) > 0 {
				return m, func() tea.Msg {
					c := exec.Command("docker", "exec", "-it", m.table.SelectedRow()[0], "bash")
					c.Stdin = os.Stdin
					c.Stdout = os.Stdout
					c.Stderr = os.Stderr
					// Temporarily suspend the program to run the command
					if output, err := c.CombinedOutput(); err != nil {
						fmt.Printf("Error executing container: %s\nOutput: %s\n", err, string(output))
					}
					return nil
				}
			}
		case "s":
			if len(m.table.SelectedRow()) > 0 {
				return m, func() tea.Msg {
					c := exec.Command("docker", "stop", m.table.SelectedRow()[0])
					if output, err := c.CombinedOutput(); err != nil {
						fmt.Printf("Error stopping container: %s\nOutput: %s\n", err, string(output))
					}
					return nil
				}
			}
		case "r":
			if len(m.table.SelectedRow()) > 0 {
				return m, func() tea.Msg {
					fmt.Printf("Starting container: '%s'\n", m.table.SelectedRow()[0]) // Debugging line
					c := exec.Command("docker", "restart", fmt.Sprintf("'%s'", m.table.SelectedRow()[0]))
					if output, err := c.CombinedOutput(); err != nil {
						fmt.Printf("Error running container: %s\nOutput: %s\n", err, string(output))
					}
					return nil
				}
			}
		}
	}
	m.table, cmd = m.table.Update(msg)
	return m, cmd
}

func (m model) View() string {
	return baseStyle.Render(m.table.View()) + "\n"
}

func main() {
	output, err := getDockerPSOutput()
	if err != nil {
		fmt.Println("Error running docker ps:", err)
		os.Exit(1)
	}

	dockerPSList := parseDockerPSOutput(output)
	rows := dockerPSToTableRows(dockerPSList)

	columns := []table.Column{
		{Title: "Container ID", Width: 20},
		{Title: "Image", Width: 20},
		{Title: "Command", Width: 20},
		{Title: "Created", Width: 20},
		{Title: "Status", Width: 20},
		{Title: "Ports", Width: 10},
		{Title: "Names", Width: 15},
	}

	t := table.New(
		table.WithColumns(columns),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(7),
	)

	s := table.DefaultStyles()
	s.Header = s.Header.
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240")).
		BorderBottom(true).
		Bold(false)

	// Apply styles conditionally based on the Ports column
	for i, row := range rows {
		if row[5] == "" {
			// Change the row style for stopped containers
			grayStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("8"))
			for j := range row {
				row[j] = grayStyle.Render(row[j])
			}
			rows[i] = row
		}
	}

	t.SetStyles(s)

	m := model{t}
	if _, err := tea.NewProgram(m).Run(); err != nil {
		fmt.Println("Error running program:", err)
		os.Exit(1)
	}
}
