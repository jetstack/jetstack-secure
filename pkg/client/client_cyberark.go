package client

import (
	"github.com/jetstack/preflight/pkg/internal/cyberark/dataupload"
)

type CyberArkClient = dataupload.CyberArkClient
type CyberArkClientOptions = dataupload.Options

var NewCyberArkClient = dataupload.NewCyberArkClient
