package v6

import "webtyp.com/network"

func accessToList(access network.Access) string {
	switch access {
	case network.AccessLocal:
		return ListLocal
	case network.AccessInternetFiltered:
		return ListInternetFiltered
	case network.AccessInternet:
		return ListInternet
	default:
		return ""
	}
}

func listToAccess(list string) network.Access {
	switch list {
	case ListLocal:
		return network.AccessLocal
	case ListInternetFiltered:
		return network.AccessInternetFiltered
	case ListInternet:
		return network.AccessInternet
	default:
		return network.AccessLocal // default fallback
	}
}
