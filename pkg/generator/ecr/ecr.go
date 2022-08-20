/*
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

package ecr

import (
	"context"
	"fmt"
	"strconv"

	"github.com/aws/aws-sdk-go/service/ecr"
	genv1alpha1 "github.com/external-secrets/external-secrets/apis/generators/v1alpha1"
	awsauth "github.com/external-secrets/external-secrets/pkg/provider/aws/auth"
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type Generator struct{}

const (
	errNoSpec     = "no config spec provided"
	errParseSpec  = "unable to parse spec: %w"
	errCreateSess = "unable to create aws session: %w"
	errGetToken   = "unable to get authorization token: %w"
)

func (g *Generator) Generate(ctx context.Context, jsonSpec *apiextensions.JSON, kube client.Client, namespace string) (map[string][]byte, error) {
	if jsonSpec == nil {
		return nil, fmt.Errorf(errNoSpec)
	}
	res, err := parseSpec(jsonSpec.Raw)
	if err != nil {
		return nil, fmt.Errorf(errParseSpec, err)
	}
	sess, err := awsauth.NewGeneratorSession(
		ctx,
		res.Spec.Auth,
		res.Spec.Role,
		res.Spec.Region,
		kube,
		namespace,
		awsauth.DefaultSTSProvider,
		awsauth.DefaultJWTProvider)
	if err != nil {
		return nil, fmt.Errorf(errCreateSess, err)
	}
	client := ecr.New(sess)
	out, err := client.GetAuthorizationToken(&ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return nil, fmt.Errorf(errGetToken, err)
	}
	if len(out.AuthorizationData) != 1 {
		return nil, fmt.Errorf("unexpected number of authorization tokens. expected 1, found %d", len(out.AuthorizationData))
	}

	exp := out.AuthorizationData[0].ExpiresAt.UTC().Unix()
	return map[string][]byte{
		"authorization_token": []byte(*out.AuthorizationData[0].AuthorizationToken),
		"proxy_endpoint":      []byte(*out.AuthorizationData[0].ProxyEndpoint),
		"expires_at":          []byte(strconv.FormatInt(exp, 10)),
	}, nil
}

func parseSpec(data []byte) (*genv1alpha1.ECRAuthorizationToken, error) {
	var spec genv1alpha1.ECRAuthorizationToken
	err := json.Unmarshal(data, &spec)
	return &spec, err
}

func init() {
	genv1alpha1.Register(genv1alpha1.ECRAuthorizationTokenKind, &Generator{})
}