package ui

import (
	"fmt"
	"os"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

// TestPreview prints screens to stdout when KINGPIN_PREVIEW is set. It is a
// development aid, not an assertion.
func TestPreview(t *testing.T) {
	if os.Getenv("KINGPIN_PREVIEW") == "" {
		t.Skip("set KINGPIN_PREVIEW=1 to print screens")
	}
	m := newTestModel(t, 100, 30)
	for i := 0; i < 12; i++ {
		m.w.Player.Stock[m.w.Products[i%3]] += 30
		m.Update(key("s"))
		m.Update(key("enter"))
		m.Update(key("enter"))
		m.Update(key(fmt.Sprint(1 + i%3)))
		m.Update(key("enter"))
		if m.mode != modePlay {
			t.Fatalf("dialog stuck at step %d: %s", m.dlg.step, m.dlg.err)
		}
		m.Update(key("n"))
		m.Update(key("enter"))
	}
	fmt.Println("---- DASHBOARD 100x30")
	fmt.Println(m.View())
	m.Update(key("2"))
	fmt.Println("---- MARKET")
	fmt.Println(m.View())
	m.Update(key("4"))
	m.w.Player.DirtyCash += 2000
	m.Update(key("h"))
	m.Update(key("j"))
	m.Update(key("h"))
	m.w.Crew.LastSkim = m.w.Day
	fmt.Println("---- CREW")
	fmt.Println(m.View())
	m.Update(key("5"))
	fmt.Println("---- MAP")
	fmt.Println(m.View())
	m.Update(key("c"))
	fmt.Println("---- POST PICKER")
	fmt.Println(m.View())
	m.Update(key("j"))
	m.Update(key("enter"))
	m.Update(key("e"))
	m.Update(key("enter"))
	fmt.Println("---- MAP AFTER POSTING")
	fmt.Println(m.View())
	m.Update(key("esc"))
	m.Update(key("6"))
	m.w.Player.DirtyCash += 20000
	fmt.Println("---- UPGRADES")
	fmt.Println(m.View())
	m.Update(key("enter"))
	fmt.Println("---- UPGRADE CONFIRM")
	fmt.Println(m.View())
	m.Update(key("y"))
	m.Update(key("j"))
	fmt.Println("---- UPGRADES AFTER BUYING")
	fmt.Println(m.View())
	m.Update(key("4"))
	m.Update(key("f"))
	fmt.Println("---- FIRE CONFIRM")
	fmt.Println(m.View())
	m.Update(key("y"))
	m.Update(key("n"))
	fmt.Println("---- REPORT WITH CREW")
	fmt.Println(m.View())
	m.Update(key("enter"))
	m.Update(key("1"))
	m.Update(key("r"))
	fmt.Println("---- REPORT")
	fmt.Println(m.View())
	m.Update(key("enter"))
	m.Update(key("s"))
	m.Update(key("enter"))
	m.Update(key("enter"))
	m.Update(key("3"))
	fmt.Println("---- SELL DIALOG")
	fmt.Println(m.View())
	m.Update(key("esc"))
	m.Update(key("esc"))
	m.Update(key("esc"))
	m.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	m.Update(key("1"))
	fmt.Println("---- DASHBOARD 80x24")
	fmt.Println(m.View())
	m.Update(key("5"))
	fmt.Println("---- MAP 80x24")
	fmt.Println(m.View())
	m.Update(key("4"))
	fmt.Println("---- CREW 80x24")
	fmt.Println(m.View())
	m.Update(key("6"))
	fmt.Println("---- UPGRADES 80x24")
	fmt.Println(m.View())
}
