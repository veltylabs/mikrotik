package routeros

// Record is one reply sentence: property name -> value.
type Record map[string]string

// Reply is the full answer to one command.
type Reply struct {
	Records []Record // the !re sentences
	Done    Record   // the !done sentence (holds "ret" after an add)
}

// Commander runs one API command, e.g.
// Run("/ip/dhcp-server/lease/print") or
// Run("/ip/dhcp-server/lease/add", "=mac-address=…", "=address=…").
type Commander interface {
	Run(sentence ...string) (Reply, error)
}

// PropRet is the property of Done that carries the id returned by "add".
const PropRet = "ret"
