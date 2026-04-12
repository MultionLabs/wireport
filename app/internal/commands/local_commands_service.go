package commands

import (
	"wireport/internal/joinrequests"
	"wireport/internal/jointokens"
	nodes "wireport/internal/nodes"
	"wireport/internal/publicservices"
)

type LocalCommandsService struct {
	NodesRepository          *nodes.Repository
	PublicServicesRepository *publicservices.Repository
	JoinRequestsRepository   *joinrequests.Repository
	JoinTokensRepository     *jointokens.Repository
}
