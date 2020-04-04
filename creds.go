package oracle

import (
	"strings"

	"github.com/hashicorp/vault/sdk/database/dbplugin"
	"github.com/hashicorp/vault/sdk/database/helper/credsutil"
)

// oracleCredentialsProducer implements CredentialsProducer.
type oracleCredentialsProducer struct {
	credsutil.SQLCredentialsProducer
}

func (ocp *oracleCredentialsProducer) GenerateUsername(config dbplugin.UsernameConfig) (string, error) {
	username, err := ocp.SQLCredentialsProducer.GenerateUsername(config)
	if err != nil {
		return "", err
	}

	username = strings.Replace(username, "-", "_", -1)
	username = strings.Replace(username, ".", "_", -1)
	username = "c##" + username
	return strings.ToLower(username), nil
}

func (ocp *oracleCredentialsProducer) GeneratePassword() (string, error) {
	password, err := ocp.SQLCredentialsProducer.GeneratePassword()
	if err != nil {
		return "", err
	}

	password = strings.Replace(password, "-", "_", -1)
	return password, nil
}
