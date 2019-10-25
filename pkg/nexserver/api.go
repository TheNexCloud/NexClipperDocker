package nexserver

import (
	"encoding/json"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"log"
	"strings"
	"time"
)

func (s *NexServer) SetupApiHandler() {
	gin.SetMode("release")
	router := gin.Default()

	config := cors.DefaultConfig()
	config.AllowOrigins = []string{"*"}
	config.AllowMethods = []string{"*"}
	config.AllowHeaders = []string{"*"}
	config.AllowCredentials = true

	router.Use(cors.New(config))

	router.GET("/api/health", s.ApiHealth)
	clusters := router.Group("/api/clusters")
	{
		clusters.GET("", s.ApiClusterList)
		clusters.GET("/:cid/agents", s.ApiAgentList)
		clusters.GET("/:cid/nodes", s.ApiNodeList)
	}
	snapshot := router.Group("/api/snapshot")
	{
		snapshot.GET("/:cid/nodes", s.ApiSnapshotNodes)
		snapshot.GET("/:cid/nodes/:nodeId", s.ApiSnapshotNodes)
	}
	metrics := router.Group("/api/metrics")
	{
		metrics.GET("/:cid/nodes", s.ApiMetricsNodes)
		metrics.GET("/:cid/nodes/:nodeId", s.ApiMetricsNodes)
	}
	router.GET("/api/agents", s.ApiAgentListAll)
	router.GET("/api/nodes", s.ApiNodeListAll)

	go func() {
		err := router.Run(fmt.Sprintf("%s:%d", s.config.Server.BindAddress, s.config.Server.ApiPort))
		if err != nil {
			log.Printf("failed api handler: %v\n", err)
		}
	}()
}

func (s *NexServer) ApiResponseJson(c *gin.Context, code int, status, message string) {
	c.JSON(code, gin.H{
		"status":  status,
		"message": message,
	})
}

func (s *NexServer) ApiHealth(c *gin.Context) {
	err := s.db.DB().Ping()
	if err != nil {
		s.ApiResponseJson(c, 500, "bad", "DB connection failed")
	} else {
		s.ApiResponseJson(c, 200, "ok", "")
	}
}

func (s *NexServer) ApiClusterList(c *gin.Context) {
	var clusters []Cluster

	result := s.db.Find(&clusters)
	if result.Error != nil {
		s.ApiResponseJson(c, 500, "bad",
			fmt.Sprintf("failed to get data: %v", result.Error))
		return
	}

	type ClusterItem struct {
		Id   uint   `json:"id"`
		Name string `json:"name"`
	}
	var items []ClusterItem

	for _, cluster := range clusters {
		items = append(items, ClusterItem{
			Id:   cluster.ID,
			Name: cluster.Name,
		})
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "",
		"data":    items,
	})
}

func (s *NexServer) ApiAgentList(c *gin.Context) {
	cId := c.Param("cid")
	if cId == "" {
		s.ApiResponseJson(c, 404, "bad", "invalid cluster id")
		return
	}

	var agents []Agent

	result := s.db.Where("cluster_id=?", cId).Find(&agents)
	if result.Error != nil {
		s.ApiResponseJson(c, 500, "bad",
			fmt.Sprintf("failed to get data: %v", result.Error))
		return
	}

	type AgentItem struct {
		Id      uint   `json:"id"`
		Version string `json:"version"`
		Ip      string `json:"ip"`
		Online  bool   `json:"online"`
	}
	var items []AgentItem

	for _, agent := range agents {
		items = append(items, AgentItem{
			Id:      agent.ID,
			Version: agent.Version,
			Ip:      agent.Ipv4,
			Online:  agent.Online,
		})
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "",
		"data":    items,
	})
}

func (s *NexServer) ApiAgentListAll(c *gin.Context) {
	rows, err := s.db.Table("agents").
		Select("agents.id, agents.version, agents.ipv4, agents.online, clusters.name").
		Joins("left join clusters on agents.cluster_id=clusters.id").Rows()
	if err != nil {
		s.ApiResponseJson(c, 500, "bad",
			fmt.Sprintf("failed to get data: %v", err))
		return
	}

	type AgentItem struct {
		Id      uint   `json:"id"`
		Version string `json:"version"`
		Ip      string `json:"ip"`
		Online  bool   `json:"online"`
	}
	clusterMap := make(map[string][]*AgentItem)

	var id uint
	var version string
	var ip string
	var online bool
	var clusterName string
	for rows.Next() {
		err := rows.Scan(&id, &version, &ip, &online, &clusterName)
		if err != nil {
			continue
		}
		_, found := clusterMap[clusterName]
		if !found {
			clusterMap[clusterName] = make([]*AgentItem, 0)
		}

		items := clusterMap[clusterName]
		items = append(items, &AgentItem{
			Id:      id,
			Version: version,
			Ip:      ip,
			Online:  online,
		})
		clusterMap[clusterName] = items
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "",
		"data":    clusterMap,
	})
}

func (s *NexServer) ApiNodeList(c *gin.Context) {
	cId := c.Param("cid")
	if cId == "" {
		s.ApiResponseJson(c, 404, "bad", "invalid cluster id")
		return
	}

	var nodes []Node

	result := s.db.Where("cluster_id=?", cId).Find(&nodes)
	if result.Error != nil {
		s.ApiResponseJson(c, 500, "bad",
			fmt.Sprintf("failed to get data: %v", result.Error))
		return
	}

	type NodeItem struct {
		Id              uint   `json:"id"`
		Host            string `json:"host"`
		Ip              string `json:"ip"`
		Os              string `json:"os"`
		Platform        string `json:"platform"`
		PlatformFamily  string `json:"platform_family"`
		PlatformVersion string `json:"platform_version"`
		AgentId         uint   `json:"agent_id"`
	}
	var items []NodeItem

	for _, node := range nodes {
		items = append(items, NodeItem{
			Id:              node.ID,
			Host:            node.Host,
			Ip:              node.Ipv4,
			Os:              node.Os,
			Platform:        node.Platform,
			PlatformFamily:  node.PlatformFamily,
			PlatformVersion: node.PlatformVersion,
			AgentId:         node.AgentID,
		})
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "",
		"data":    items,
	})
}

func (s *NexServer) ApiNodeListAll(c *gin.Context) {
	rows, err := s.db.Table("nodes").
		Select("nodes.id, nodes.host, nodes.ipv4, nodes.os, " +
			"nodes.platform, nodes.platform_family, nodes.platform_version, nodes.agent_id, clusters.name").
		Joins("left join clusters on nodes.cluster_id=clusters.id").Rows()
	if err != nil {
		s.ApiResponseJson(c, 500, "bad",
			fmt.Sprintf("failed to get data: %v", err))
		return
	}

	type NodeItem struct {
		Id              uint   `json:"id"`
		Host            string `json:"host"`
		Ip              string `json:"ip"`
		Os              string `json:"os"`
		Platform        string `json:"platform"`
		PlatformFamily  string `json:"platform_family"`
		PlatformVersion string `json:"platform_version"`
		AgentId         uint   `json:"agent_id"`
	}
	clusterMap := make(map[string][]*NodeItem)

	var clusterName string
	for rows.Next() {
		nodeItem := NodeItem{}
		err := rows.Scan(&nodeItem.Id, &nodeItem.Host, &nodeItem.Ip,
			&nodeItem.Os, &nodeItem.Platform, &nodeItem.PlatformFamily, &nodeItem.PlatformVersion,
			&nodeItem.AgentId, &clusterName)
		if err != nil {
			continue
		}
		_, found := clusterMap[clusterName]
		if !found {
			clusterMap[clusterName] = make([]*NodeItem, 0)
		}

		items := clusterMap[clusterName]
		items = append(items, &nodeItem)
		clusterMap[clusterName] = items
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "",
		"data":    clusterMap,
	})
}

func (s *NexServer) ApiSnapshotNodes(c *gin.Context) {
	cId := c.Param("cid")
	if cId == "" {
		s.ApiResponseJson(c, 404, "bad", "invalid cluster id")
		return
	}

	nodeId := c.Param("nodeId")
	nodeQuery := ""
	if nodeId != "" {
		nodeQuery = fmt.Sprintf("AND m2.node_id=%s", nodeId)
	}

	rows, err := s.db.Raw(fmt.Sprintf(`
SELECT nodes.host as node, nodes.id, m1.ts, m1.value, metric_names.name, metric_labels.label
FROM metric_names, metric_labels, nodes, metrics m1
JOIN (
    SELECT m2.node_id, MAX(ts) ts
    FROM metrics m2
    WHERE m2.node_id != 0 AND m2.ts >= NOW() - interval '15 seconds' %s
    GROUP BY m2.node_id) newest
ON newest.node_id=m1.node_id AND newest.ts=m1.ts
WHERE m1.name_id=metric_names.id AND m1.node_id=nodes.id AND m1.label_id=metric_labels.id;`, nodeQuery)).Rows()
	if err != nil {
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("failed to get data: %v", err))
		return
	}

	type NodeMetric struct {
		Node        string    `json:"node"`
		NodeId      uint      `json:"node_id"`
		Ts          time.Time `json:"ts"`
		Value       float64   `json:"value"`
		MetricName  string    `json:"metric_name"`
		MetricLabel string    `json:"metric_label"`
	}

	results := make(map[string][]NodeMetric)

	for rows.Next() {
		var nodeMetric NodeMetric

		err := rows.Scan(&nodeMetric.Node, &nodeMetric.NodeId, &nodeMetric.Ts, &nodeMetric.Value,
			&nodeMetric.MetricName, &nodeMetric.MetricLabel)
		if err != nil {
			continue
		}

		nodeMetrics, found := results[nodeMetric.Node]
		if !found {
			results[nodeMetric.Node] = make([]NodeMetric, 0, 16)
		}

		nodeMetrics = append(nodeMetrics, nodeMetric)
		results[nodeMetric.Node] = nodeMetrics
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "",
		"data":    results,
	})
}

type Query struct {
	Timezone    string   `json:"timezone"`
	MetricNames []string `json:"metricNames"`
	DateRange   []string `json:"dateRange"`
	Granularity string   `json:"granularity"`
}

func (s *NexServer) findMetricIdByNames(names []string) []string {
	if len(names) == 0 {
		return []string{}
	}

	quotedNames := make([]string, 0, len(names))
	for _, name := range names {
		quotedNames = append(quotedNames, fmt.Sprintf("'%s'", name))
	}
	namesQuery := strings.Join(quotedNames, ",")
	q := fmt.Sprintf("SELECT id FROM metric_names WHERE name IN (%s)", namesQuery)

	rows, err := s.db.Raw(q).Rows()
	if err != nil {
		log.Printf("failed to get metric names: %v", err)
		return []string{}
	}

	results := make([]string, 0, 4)
	var id string

	for rows.Next() {
		err := rows.Scan(&id)
		if err != nil {
			log.Printf("failed to get metric names id: %v", err)
			continue
		}

		results = append(results, id)
	}

	return results
}

func (s *NexServer) ApiMetricsNodes(c *gin.Context) {
	cId := c.Param("cid")
	if cId == "" {
		s.ApiResponseJson(c, 404, "bad", "invalid cluster id")
		return
	}

	nodeId := c.Param("nodeId")
	nodeQuery := ""
	if nodeId != "" {
		nodeQuery = fmt.Sprintf("AND metrics.node_id=%s", nodeId)
	}

	q := c.DefaultQuery("query", "")
	if q == "" {
		s.ApiResponseJson(c, 404, "bad", "invalid query")
		return
	}

	var query Query
	err := json.Unmarshal([]byte(q), &query)
	if err != nil {
		s.ApiResponseJson(c, 404, "bad", "invalid query")
		return
	}

	metricNameIds := s.findMetricIdByNames(query.MetricNames)
	metricNameQuery := ""
	if len(metricNameIds) > 0 {
		metricNameQuery = fmt.Sprintf(" AND metrics.name_id IN (%s)", strings.Join(metricNameIds, ","))
	}
	granularity := query.Granularity
	if granularity == "" {
		granularity = "minute"
	}

	metricQuery := fmt.Sprintf(`
SELECT nodes.host as node, nodes.id as node_id, value, bucket,
       metric_names.name, metric_labels.label FROM
    (SELECT metrics.node_id as node_id, avg(value) as value,
            metrics.name_id, metrics.label_id,
           date_trunc('%s', ts) as bucket
    FROM metrics
    WHERE ts >= '%s' AND ts < '%s' AND metrics.cluster_id=%s %s %s
    GROUP BY bucket, metrics.node_id, metrics.name_id, metrics.label_id)
        as metrics_bucket, nodes, metric_names, metric_labels
WHERE
    metrics_bucket.node_id=nodes.id AND
      metrics_bucket.name_id=metric_names.id AND
      metrics_bucket.label_id=metric_labels.id
ORDER BY bucket;
`, granularity, query.DateRange[0], query.DateRange[1], cId, nodeQuery, metricNameQuery)

	rows, err := s.db.Raw(metricQuery).Rows()
	if err != nil {
		log.Printf("failed to get metric data: %v", err)
		s.ApiResponseJson(c, 500, "bad", fmt.Sprintf("unexpected error: %v", err))
		return
	}

	type MetricItem struct {
		Node        string  `json:"node"`
		NodeId      uint    `json:"node_id"`
		Value       float64 `json:"value"`
		Bucket      string  `json:"bucket"`
		MetricName  string  `json:"metric_name"`
		MetricLabel string  `json:"metric_label"`
	}
	results := make([]MetricItem, 0, 16)

	for rows.Next() {
		var item MetricItem

		err := rows.Scan(&item.Node, &item.NodeId, &item.Value, &item.Bucket, &item.MetricName, &item.MetricLabel)
		if err != nil {
			log.Printf("failed to get record: %v", err)
			continue
		}

		results = append(results, item)
	}

	c.JSON(200, gin.H{
		"status":  "ok",
		"message": "",
		"data":    results,
	})
}