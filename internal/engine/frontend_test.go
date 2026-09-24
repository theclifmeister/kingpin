package engine_test

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

// worldReads is every World method the TUI calls: each reads the world
// and none changes it. An action is a session command (commands.go),
// never a World method a front end calls itself.
var worldReads = []string{
	"AssetLive", "AssetLost", "AtPeaceWith", "Available", "BestSupplier", "Bound", "BuyMarkup", "Campaigning", "CanBuyIn",
	"CanCrown", "CanUndercut", "CanVanish", "Capacity", "Cash", "Checkpoint", "City", "CityName",
	"CityOf", "Cold", "Contested", "ContestedBy", "Contract", "ContractsDue", "ContractsIn", "Corner",
	"DealWith", "DeedValue", "Deeds", "DeedsIn", "DelegatedOrder", "DelegatedSupplied", "Deliverable", "Demand",
	"Describe", "DrivenRoute", "ExportOrder", "ExportsOut", "Faction", "FactionIndex", "FactionName", "FallGuyLeft", "FavourCalled", "Float", "Free",
	"Front", "FrontCity", "Glut", "GuardOf", "Held", "Here", "HomageDeals", "Home", "House", "HousesIn",
	"LeadName", "Lot", "MaxBuy", "Missing", "NextDoor", "Order", "Owed", "Owns", "PostOf", "Product",
	"ProductName", "Quality", "QueuedDelivery", "Quote", "ReachedOn", "ReignDay", "ReservedToday", "Rival",
	"RivalHeld", "RivalHeldBy", "Route", "RouteClosed", "Score", "Side", "SplitLinesWith", "Stage",
	"StagePending", "Stance", "StashOf", "Stashed", "Stock", "StockIn", "Street", "StreetCapacity",
	"StreetQuality", "StreetSupplier", "Supplied", "SuppliedToday", "Supplier", "SuppliersIn", "Tier", "TierName",
	"TotalStock", "Undercutting", "WarHasGround", "WholesaleSupplier", "Worked", "WorkedIn", "YourStanding",
}

// sceneWorld is the file whose World calls are on a world of its own:
// the title's demo builds a throwaway run to draw a scene over (#153)
// and never touches the one being played.
const sceneWorld = "demo.go"

// TestUIActsThroughTheSession (#297): the TUI is a front end like any
// other. It reads the sims through engine.Rules, never the sims
// themselves (no import of internal/sim, no Session.Sims), and it acts
// on the run only through the session's commands: every World method it
// calls is in worldReads. A World action called from the TUI fails here
// by name; the fix is a command in commands.go (or, for a method that
// only reads, a line in worldReads), so no action is the TUI's alone.
func TestUIActsThroughTheSession(t *testing.T) {
	t.Parallel()
	dir := filepath.Join("..", "ui")
	names, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	fset := token.NewFileSet()
	var files []*ast.File
	var offenders []string
	for _, name := range names {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		for _, imp := range f.Imports {
			path, _ := strconv.Unquote(imp.Path.Value)
			if strings.HasSuffix(path, "/internal/sim") || strings.Contains(path, "/internal/sim/") {
				offenders = append(offenders, fset.Position(imp.Pos()).String()+": imports "+path+"; read the sims through engine.Rules")
			}
		}
		files = append(files, f)
	}
	if len(files) < 50 {
		t.Fatalf("only %d files parsed; the directory is wrong", len(files))
	}
	info := &types.Info{Selections: map[*ast.SelectorExpr]*types.Selection{}}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil)}
	if _, err := conf.Check("ui", fset, files, info); err != nil {
		t.Fatal(err)
	}
	reads := map[string]bool{}
	for _, r := range worldReads {
		reads[r] = true
	}
	seen := map[string]bool{}
	for sel, s := range info.Selections {
		if s.Kind() != types.MethodVal {
			continue
		}
		recv := types.TypeString(s.Recv(), nil)
		name := sel.Sel.Name
		at := fset.Position(sel.Pos())
		switch {
		case strings.HasSuffix(recv, "/engine.Session") && name == "Sims":
			offenders = append(offenders, at.String()+": Session.Sims; read the sims through engine.Rules")
		case strings.HasSuffix(recv, "/game.World"):
			if filepath.Base(at.Filename) == sceneWorld {
				continue
			}
			seen[name] = true
			if !reads[name] {
				offenders = append(offenders, at.String()+": World."+name+"; act through a session command (engine/commands.go), or list it in worldReads if it only reads")
			}
		}
	}
	sort.Strings(offenders)
	if len(offenders) > 0 {
		t.Errorf("the TUI goes round the session:\n  %s", strings.Join(offenders, "\n  "))
	}
	for _, r := range worldReads {
		if !seen[r] {
			t.Errorf("worldReads lists %s, which the TUI no longer calls; take it out", r)
		}
	}
}
