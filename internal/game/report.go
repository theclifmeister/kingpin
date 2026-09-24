package game

// Headline is a journal entry.
type Headline struct {
	Day    int
	Source string
	Text   string
}

// DayReport is what the player reads in the morning.
type DayReport struct {
	Day        int
	Incident   []string // the world's incident this morning (#44): first in the report, before the tier
	Unlocked   []string // gates crossed this morning (#148): first in the report
	Prices     []string
	Sales      []string
	Heat       []string
	Crew       []string
	Territory  []string
	Shipments  []string
	Law        []string // elections, a new chief, pressure bands crossed, what you gave a city
	Intel      []string // what was learnt tonight (#45): the facts filed, a spy's night, a lie that bit
	Money      []string
	Upgrades   []string
	Tier       []string // the tier entered this morning (#147), first in the report
	News       []string
	CashBefore int
	CashAfter  int
	Flow       CashFlow // the night's money by category, dirty and clean (#351); CashBefore and CashAfter are its opening and closing
	Lead       []Line   // the night's biggest changes, biggest first (#354): the report opens with them under TODAY
}

// Line is one line of the morning's lead (#354): what changed, in
// words, and what answers it. Kind is the headlines.toml [digest] key
// that scored it; Member, Corner, City and House are the ids the act's
// subject names, as an alert's are (engine.Alert), zero where it names
// none.
type Line struct {
	Kind   string
	Text   string
	Act    Act
	Member int
	Corner string
	City   string
	House  string
}

// Act is what answers a line (#352): the screen that deals with it, the
// dialog it opens there, and which of the line's own fields names what
// it opens on. The engine's alerts carry it (engine.Act is this type)
// and so does the morning's lead (#354); each front end maps it onto
// its own screens and dialogs, and one that lacks the screen shows the
// words alone. An act opens nothing that spends.
type Act struct {
	Screen  string `json:"screen"`            // a Screen* name
	Mode    string `json:"mode,omitempty"`    // a Mode* name, or "" for the screen alone
	Subject string `json:"subject,omitempty"` // an On* name: the field that holds the id, or "" for none
}

// The screens an act lands on, by the TUI's names for its tabs.
const (
	ScreenDashboard = "dashboard"
	ScreenMarket    = "market"
	ScreenCrew      = "crew"
	ScreenMap       = "map"
	ScreenLedger    = "ledger"
	ScreenRivals    = "rivals"
)

// ModePost is the one dialog an act opens: the post picker on the
// line's Corner.
const ModePost = "post"

// The subjects an act opens on, each the field of that name on the
// alert or the line.
const (
	OnMember   = "member"
	OnCorner   = "corner"
	OnContract = "contract"
	OnSupplier = "supplier"
	OnHouse    = "house"
	OnCity     = "city"
)

// Ending records how a run finished: the day, the cause (one of
// content.Causes: indicted, arrested, broke, retired, businessman,
// kingpin, betrayed, taken_out, vanished; #49, docs/endings.md), the
// peak cash, and Who for a cause with a name in it (the lieutenant who
// flipped, the faction's leader who took the last corner or broke the
// deal). The sim that owns a cause writes it through World.End in its
// step; the summary reads it and the score off Stats.
type Ending struct {
	Day      int
	Cause    string
	PeakCash int
	Who      string
}

// Stats are lifetime counters for the run summary.
type Stats struct {
	PeakCash       int
	TotalRevenue   int
	UnitsSold      int
	Raids          int
	Stings         int
	Wages          int
	Skimmed        int
	Robbed         int
	Strikes        int // enforcers sent against a rival corner
	Boosts         int // enforcers sent for a rival corner's takings (#70)
	Boosted        int // dirty cash they took off it
	Scouts         int // looks bought at the rival's books
	Tips           int // tips you gave the police on a rival corner
	RivalRaids     int // rival corners the police took on your tips
	Poached        int // heads of the rival's muscle you paid to go home
	CornersWon     int // rival corners taken by force
	CornersLost    int // corners the rival took from you
	Laundered      int // dirty cash washed clean
	Seized         int // clean cash lost to audits
	Informants     int // crew who turned on you
	Defections     int // crew who went over to the rival
	Investigations int
	Shipments      int // shipments sent
	Shipped        int // units sent over a route
	Seizures       int // shipments the police took on the road
	SeizedOnRoad   int // units lost to them
	Deals          int // deals struck with the rival, either way
	DealsRefused   int // proposals it turned down
	Betrayals      int // deals you broke
	BetrayedBy     int // deals it broke
	Tribute        int // dirty cash paid the rival in tribute
	Homage         int // dirty cash the factions paid you in homage (#43)
	CrewPoached    int // your crew poached by a faction (#43)
	Absorbed       int // factions absorbed by another (#43)
	Fragmented     int // factions that lost their leader (#43)
	Moves          int // factions that sent scouts to a city where you earn (#341)
	Expanded       int // of those, the ones that arrived there
	Withdrew       int // of those, the ones whose scouts went home
	Cuts           int // dirty cash the lieutenants kept as their cut, and the crew's cut on your standing orders (#114)
	Walked         int // lieutenants who walked with their city
	Funded         int // clean cash given to the cities (#41)
	Backed         int // clean cash put behind DA campaigns (#193)
	Campaigns      int // campaigns backed, one a city an election
	CampaignsWon   int // of those, the ticket that won
	Bribes         int // envelopes paid to the chief and the DA (#42)
	Bribed         int // dirty cash in them
	Backfires      int // of those, the ones that blew up
	Checkpoints    int // checkpoints and customs deals bought
	CheckpointCash int // dirty cash they cost
	Leads          int // leads the DA's office picked up from your envelopes
	Favours        int // favours called in on a bought chief (#228): responses that fell through
	Elections      int // DA elections held
	Chiefs         int // police chiefs replaced
	Contracts      int // buyers' contracts delivered in full (#71)
	ContractUnits  int // units handed over to buyers
	ContractCash   int // dirty cash the buyers paid
	ContractsShort int // contracts short at the due day
	Credit         int // dirty cash's worth of product taken on credit from the connects (#72)
	Repaid         int // what has been paid back
	LatePayments   int // debts that were short on their day
	DebtDays       int // days ended owing a connect something
	Collected      int // units a connect took from the stash for a debt
	Rent           int // clean cash paid the landlords (#73)
	HousesLost     int // houses the landlord threw you out of
	HouseUnits     int // units lost out of the houses to raids and robberies
	Earned         int // clean cash the levelled fronts earned on their own (#192)
	Invested       int // clean cash put into the fronts' levels
	Cut            int // units the cuts added (#47)
	CutCost        int // dirty cash the cuts cost
	Cooked         int // units the chemist cooked
	CookCost       int // dirty cash the precursors cost
	Overdoses      int // overdoses on your corners
	Reserved       int // clean cash moved offshore, after the fee (#195)
	Fees           int // what the account kept of it
	Bodies         int // the dead on your corners, both sides (#46): the score's divisor (#49); never decreases
	Fallen         int // of those, yours
	Arrests        int // crew put in a cell
	Bails          int // ... and walked out of it on your clean cash
	BailCash       int // what that cost
	Deeds          int // blocks bought (#194)
	DeedCash       int // clean cash they cost
	DeedRent       int // clean cash the blocks paid back
	Taxed          int // dirty cash the free corners paid you for the right to work them (#231)
	DeedsSeized    int // deeds the DA took (the forfeiture)
	Wounded        int // crew shot and laid up
	Retired        int // crew who retired

	// Intel (#45).
	CopsPaid    int // cops paid for a word
	CopCash     int // what they were paid
	Spies       int // crew sent under
	SpiesFound  int // ... and found out: shot or turned
	SpiesShot   int // ... and shot for it
	Reports     int // what the spies filed
	Lures       int // facts a faction fed you
	Bitten      int // ... that you acted on
	PeakClean   int // the most clean cash held at once (#48): the high-water mark the assets unlock on, stamped by the clock beside PeakCash
	Assets      int // assets bought (#48)
	AssetCash   int // clean cash they cost
	AssetsLost  int // assets the task force seized or the police found
	AssetUpkeep int // clean cash the assets' upkeep took
	TaskForces  int // task forces that came
	Score       int // the run's score (#49), stamped by End: the offshore account over one plus the bodies; the pile left behind is printed, never scored
}
