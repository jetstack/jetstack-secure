package client

import (
	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
)

type CyberArkClient = dataupload.CyberArkClient

var NewCyberArkClient = dataupload.NewCyberArkClient
