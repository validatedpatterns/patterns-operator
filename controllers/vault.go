/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strconv"

	"github.com/go-errors/errors"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	utilexec "k8s.io/client-go/util/exec"
)

//
// {
// 	"unseal_keys_b64": [
// 	  "R/BL306DUjRQIHdkYYxheqFxr6PtZVEKtHaYNjFqBGq7",
// 	  "+4CYavmqRWq165WJM4DqpnEqlDnECt6q+6jSmEaJBsBA",
// 	  "tlsQ833l5k52ESK28jlZlWbegBRY+HNIJD9Yqp3cEdF6",
// 	  "ON0wQUleo+iW4r6U0EwmoOkRhezzTke09h+rxgRDPkdo",
// 	  "i1hmENQhAcq5t6WWxTR35YDAUjY1w8ry751CggPsB0Jk"
// 	],
// 	"unseal_keys_hex": [
// 	  "47f04bdf4e83523450207764618c617aa171afa3ed65510ab4769836316a046abb",
// 	  "fb80986af9aa456ab5eb95893380eaa6712a9439c40adeaafba8d298468906c040",
// 	  "b65b10f37de5e64e761122b6f239599566de801458f87348243f58aa9ddc11d17a",
// 	  "38dd3041495ea3e896e2be94d04c26a0e91185ecf34e47b4f61fabc604433e4768",
// 	  "8b586610d42101cab9b7a596c53477e580c0523635c3caf2ef9d428203ec074264"
// 	],
// 	"unseal_shares": 5,
// 	"unseal_threshold": 3,
// 	"recovery_keys_b64": [],
// 	"recovery_keys_hex": [],
// 	"recovery_keys_shares": 5,
// 	"recovery_keys_threshold": 3,
// 	"root_token": "s.lAR1G890NPBEkzRt8Ic5kBVz"
//   }
// Note: We only keep the minimum needed fields around
type VaultInitStruct struct {
	UnsealKeysHex   []string `json:"unseal_keys_hex"`
	UnsealShares    int      `json:"unseal_shares"`
	UnsealThreshold int      `json:"unseal_threshold"`
	RootToken       string   `json:"root_token"`
}

// {
// 	"type": "shamir",
// 	"initialized": false,
// 	"sealed": true,
// 	"t": 0,
// 	"n": 0,
// 	"progress": 0,
// 	"nonce": "",
// 	"version": "1.9.2",
// 	"migration": false,
// 	"recovery_seal": false,
// 	"storage_type": "file",
// 	"ha_enabled": false,
// 	"active_time": "0001-01-01T00:00:00Z"
//   }
type VaultStatus struct {
	Type        string `json:"type"`
	Initialized bool   `json:"initialized"`
	Sealed      bool   `json:"sealed"`
	T           int    `json:"t"`
	N           int    `json:"n"`
	Progress    int    `json:"progress"`
	Version     string `json:"version"`
	StorageType string `json:"storage_type"`
	HaEnabled   bool   `json:"ha_enabled"`
}

const (
	vaultSecretName   string = "vaultkeys"
	vaultNamespace    string = "vault"
	vaultPod          string = "vault-0"
	vaultContainer    string = "vault"
	operatorNamespace string = "patterns-operator-system"
)

func vaultOperatorInit(config *rest.Config, client kubernetes.Interface) (*VaultInitStruct, error) {
	stdout, stderr, err := execInPod(config, client, vaultNamespace, vaultPod, vaultContainer, []string{"vault", "operator", "init", "-format=json"})
	if err != nil {
		return nil, fmt.Errorf("%v - %s - %s", err, stdout, stderr)
	}
	var unmarshalled VaultInitStruct
	err = json.Unmarshal(stdout.Bytes(), &unmarshalled)
	if err != nil {
		return nil, fmt.Errorf("%v - %s - %s", err, stdout, stderr)
	}
	return &unmarshalled, nil
}

func createVaultSecrets(config *rest.Config, client kubernetes.Interface, vaultInitOutput *VaultInitStruct) error {
	log.Printf("Adding vault keys to secrets")
	data := map[string][]byte{
		"roottoken": []byte(vaultInitOutput.RootToken),
	}
	for index, key := range vaultInitOutput.UnsealKeysHex {
		s := "unsealhexkey_" + strconv.Itoa(index)
		data[s] = []byte(key)
	}
	data["keyscount"] = []byte(strconv.Itoa(len(vaultInitOutput.UnsealKeysHex)))

	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: vaultSecretName,
		},
		Data: data,
	}
	secretClient := client.CoreV1().Secrets(operatorNamespace)
	current, err := secretClient.Get(context.Background(), vaultSecretName, metav1.GetOptions{})
	if err != nil || current == nil {
		_, err = secretClient.Create(context.Background(), secret, metav1.CreateOptions{})
	} else {
		_, err = secretClient.Update(context.Background(), secret, metav1.UpdateOptions{})
	}
	if err != nil {
		log.Printf("Error creating secret: %s\n", err)
		return err
	}
	log.Printf("Created secret")
	return nil
}

func getVaultStructFromSecrets(config *rest.Config, client kubernetes.Interface) (*VaultInitStruct, error) {
	// If the vault is sealed we take the unseal keys in the k8s secret and use them to unseal the vault
	secretClient := client.CoreV1().Secrets(operatorNamespace)
	secret, err := secretClient.Get(context.Background(), vaultSecretName, metav1.GetOptions{})
	if err != nil || secret == nil {
		return nil, errors.New(fmt.Errorf("We called vaultUnseal but there were no secrets present: %s", err))
	}
	count, err := strconv.Atoi(string(secret.Data["keyscount"]))
	if err != nil {
		return nil, errors.New(fmt.Errorf("Converting keys count failed: %s", err))
	}
	var v VaultInitStruct
	v.RootToken = string(secret.Data["roottoken"])
	v.UnsealKeysHex = []string{}
	for i := 0; i < count; i++ {
		v.UnsealKeysHex = append(v.UnsealKeysHex, string(secret.Data["unsealhexkey_"+strconv.Itoa(i)]))
	}
	err = unsealVaultOperator(config, client, &v)
	if err != nil {
		return nil, err
	}
	return &v, nil
}

func unsealVaultOperator(config *rest.Config, client kubernetes.Interface, vaultInitOutput *VaultInitStruct) error {
	var errCount int = 0
	if len(vaultInitOutput.UnsealKeysHex) == 0 || len(vaultInitOutput.UnsealKeysHex) < vaultInitOutput.UnsealThreshold {
		return errors.New("We do not have sufficient keys to unseal the vault")
	}

	for _, key := range vaultInitOutput.UnsealKeysHex {
		_, _, err := execInPod(config, client, vaultNamespace, vaultPod, vaultContainer, []string{"vault", "operator", "unseal", key})
		if err != nil {
			errCount += 1
			log.Printf("Error while processing %s: -> %s", key, err)
		}
	}
	if errCount > 0 {
		return errors.New("Errored while calling vault operator unseal")
	}

	log.Printf("Vault successfully unsealed")
	return nil
}

func vaultStatus(config *rest.Config, client kubernetes.Interface) (*VaultStatus, error) {
	if !haveNamespace(client, vaultNamespace) {
		return nil, fmt.Errorf("'%s' namespace not found yet", vaultNamespace)
	}
	if !havePod(client, vaultNamespace, vaultPod) {
		return nil, fmt.Errorf("'%s/%s' pod not found yet", vaultNamespace, vaultPod)
	}
	log.Printf("%s/%s exists. Getting vault status:", vaultNamespace, vaultPod)
	stdout, stderr, err := execInPod(config, client, vaultNamespace, vaultPod, vaultContainer, []string{"vault", "status", "-format=json"})
	var ret int = 0
	if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
		ret = exitErr.ExitStatus()
	}
	// We only error out with rc=1, because rc=2 means sealed
	if ret == 1 {
		return nil, fmt.Errorf("Vault status error: %v - %s - %s", err, stdout.String(), stderr.String())
	}
	var unmarshalled VaultStatus
	err = json.Unmarshal(stdout.Bytes(), &unmarshalled)
	if err != nil {
		return nil, fmt.Errorf("Vault status Json parsing error: %v - %s - %s", err, stdout, stderr)
	}
	return &unmarshalled, nil
}

// vaultInitialize returns (changed bool, error)
func vaultInitialize(config *rest.Config, client kubernetes.Interface) (bool, error) {
	status, err := vaultStatus(config, client)
	if err != nil {
		return false, err
	}
	if status.Initialized {
		return false, nil
	}
	// If the vault is not initialized we call 'vault operator init -format=json' and store the unseal keys in k8s
	vaultKeys, err := vaultOperatorInit(config, client)
	if err != nil {
		return false, err
	}

	// Let's store the keys into a secret
	if err = createVaultSecrets(config, client, vaultKeys); err != nil {
		return false, err
	}
	// We correctly initialized the vault
	return true, nil
}

// vaultUnseal returns (changed bool, error)
func vaultUnseal(config *rest.Config, client kubernetes.Interface) (bool, error) {
	status, err := vaultStatus(config, client)
	if err != nil {
		return false, err
	}
	if !status.Sealed {
		return false, nil
	}
	if !status.Initialized {
		return false, fmt.Errorf("Vault is sealed but not initialized. This is a non-expected state!")
	}
	v, err := getVaultStructFromSecrets(config, client)
	if err != nil {
		return false, err
	}
	if err = unsealVaultOperator(config, client, v); err != nil {
		return false, err
	}
	return true, nil
}

// vaultLogin returns (changed bool, error)
func vaultLogin(config *rest.Config, client kubernetes.Interface) (bool, error) {
	stdout, stderr, err := execInPod(config, client, vaultNamespace, vaultPod, vaultContainer, []string{"vault", "token", "lookup"})
	// we are already logged in. Nothing else to do here
	if err == nil {
		return false, nil
	}
	var ret int = 0
	if exitErr, ok := err.(utilexec.ExitError); ok && exitErr.Exited() {
		ret = exitErr.ExitStatus()
	}
	// There has been a generic error while looking up the token
	if ret == 1 || ret > 2 {
		return false, fmt.Errorf("Generic error while looking up vault token: %s,%s", stdout, stderr)
	}
	v, err := getVaultStructFromSecrets(config, client)
	if err != nil {
		return false, err
	}
	// The user does not have a token so we must login using the root token
	stdout, stderr, err = execInPod(config, client, vaultNamespace, vaultPod, vaultContainer, []string{"vault", "login", v.RootToken})
	if err != nil {
		log.Printf("Error while logging in to the vault %s %s %s\n", stdout.String(), stderr.String(), err)
		return false, err
	}
	log.Printf("Logged into the vault successfully")
	return true, nil
}
