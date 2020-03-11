package api

import (
	"log"
	"os"
	"strings"

	"github.com/grafana/grafana/pkg/api/pluginproxy"
	"github.com/grafana/grafana/pkg/infra/metrics"
	m "github.com/grafana/grafana/pkg/models"
	"github.com/grafana/grafana/pkg/plugins"
)

//environment variable
const (
	tenantHeaderName      = "GRAFANA_TENANT_HEADER_NAME"       //tenant
	adminTenantHeaderName = "GRAFANA_ADMIN_TENANT_HEADER_NAME" //admin tenant, admin can visit all.
)

const targetURL = "/label/namespace/values"

type labelValesResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

func (hs *HTTPServer) ProxyDataSourceRequestMultiTenancy(c *m.ReqContext) {
	c.TimeRequest(metrics.MDataSourceProxyReqTimer)

	//for multitenancy
	tenantCode := strings.ToLower(c.Req.Header.Get(os.Getenv(tenantHeaderName)))
	adminTenantCode := os.Getenv(adminTenantHeaderName)
	log.Printf("tenantCode: %s, adminTenantCode: %s.\n", tenantCode, adminTenantCode)
	if strings.HasSuffix(c.Req.URL.Path, targetURL) && tenantCode != "" && tenantCode != adminTenantCode {
		log.Printf("return specified tenant,tenantCode: %s, adminTenantCode: %s.\n", tenantCode, adminTenantCode)
		result := labelValesResponse{}
		result.Status = "success"
		result.Data = append(result.Data, tenantCode)
		c.JSON(200, result)
		return
	}

	dsId := c.ParamsInt64(":id")
	ds, err := hs.DatasourceCache.GetDatasource(dsId, c.SignedInUser, c.SkipCache)
	if err != nil {
		if err == m.ErrDataSourceAccessDenied {
			c.JsonApiErr(403, "Access denied to datasource", err)
			return
		}
		c.JsonApiErr(500, "Unable to load datasource meta data", err)
		return
	}

	// find plugin
	plugin, ok := plugins.DataSources[ds.Type]
	if !ok {
		c.JsonApiErr(500, "Unable to find datasource plugin", err)
		return
	}

	// macaron does not include trailing slashes when resolving a wildcard path
	proxyPath := ensureProxyPathTrailingSlash(c.Req.URL.Path, c.Params("*"))

	proxy := pluginproxy.NewDataSourceProxy(ds, plugin, c, proxyPath, hs.Cfg)
	proxy.HandleRequest()
}
