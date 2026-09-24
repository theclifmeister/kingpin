package content

import (
	"strings"
	"testing"
)

// A rival leader is never a connect's name or a crew name (#425): the
// rival Cass took corners while the connect Cass sold at 55% of street.
func TestRivalNamesStandApart(t *testing.T) {
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := c.Names.validateApart(c.Suppliers); err != nil {
		t.Fatal(err)
	}
	n := c.Names
	n.Rivals = append([]string{c.Suppliers.Deck[0].Name}, n.Rivals...)
	if err := n.validateApart(c.Suppliers); err == nil || !strings.Contains(err.Error(), "connect") {
		t.Fatalf("a rival named for the connect %q: %v", c.Suppliers.Deck[0].Name, err)
	}
	n = c.Names
	n.Rivals = append([]string{n.Crew[0]}, n.Rivals...)
	if err := n.validateApart(c.Suppliers); err == nil {
		t.Fatalf("a rival named for the crew's %q passed", n.Crew[0])
	}
}
