package v6

const (
	ManagedMarker = "[velty]" // comment prefix of every object this package owns

	ListLocal            = "velty-local"
	ListInternetFiltered = "velty-internet-filtered"
	ListInternet         = "velty-internet"

	WANList = "WAN" // interface list that holds the Internet uplink(s); must exist on the router

	PathResource   = "/system/resource"
	PathDHCPServer = "/ip/dhcp-server"
	PathLease      = "/ip/dhcp-server/lease"
	PathFilter     = "/ip/firewall/filter"
	PathNAT        = "/ip/firewall/nat"
	PathARP        = "/ip/arp"
	PathBridge     = "/interface/bridge"
	PathIfaceList  = "/interface/list"

	StaticOnlyPool = "static-only"

	PropID           = ".id"
	PropComment      = "comment"
	PropMAC          = "mac-address"
	PropAddress      = "address"
	PropServer       = "server"
	PropAddressLists = "address-lists"
	PropDynamic      = "dynamic"
	PropStatus       = "status"
	PropHostName     = "host-name"

	CmdPrint  = "/print"
	CmdAdd    = "/add"
	CmdSet    = "/set"
	CmdRemove = "/remove"

	BaselineInternet            = "[velty] internet"
	BaselineInternetFiltered    = "[velty] internet filtered"
	BaselineBlock               = "[velty] block"
	BaselineDNSFilterUDP        = "[velty] dns filter udp"
	BaselineDNSFilterTCP        = "[velty] dns filter tcp"
)
