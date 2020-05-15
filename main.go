package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	flag "github.com/spf13/pflag"
)

const (
	refreshMinInterval = 120
)

var (
	lastRefreshTimeStamp int64
	rc                   *RedisCache
	username             = `missdeer`
	project              = `coredns-custom-build`
	projects             []string
	avs                  []*Appveyor
	token                = ``
)

func handler(c *gin.Context) {
	baseName := filepath.Base(c.Param("baseName"))

	targetLink, err := rc.GetString(baseName)
	if err != nil || targetLink == "" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}
	userAgent := c.GetHeader("User-Agent")
	if userAgent == "CoreDNSHome" {
		c.Redirect(http.StatusFound, targetLink)
		return
	}

	n := strings.Split(baseName, "-")
	slug := projects[0]
	for _, p := range projects {
		if strings.HasPrefix(p, n[0]) {
			slug = p
			break
		}
	}

	acceptLang := c.GetHeader("Accept-Language")
	if strings.Contains(acceptLang, "zh") {
		c.HTML(http.StatusOK, "index.zh.tmpl", gin.H{
			"targetLink":  targetLink,
			"username":    username,
			"projectSlug": slug,
			"project":     n[0],
		})
		return
	}
	c.HTML(http.StatusOK, "index.en.tmpl", gin.H{
		"targetLink":  targetLink,
		"username":    username,
		"projectSlug": slug,
		"project":     n[0],
	})
}

func updateLinkMap() {
	now := time.Now().Unix()
	if atomic.LoadInt64(&lastRefreshTimeStamp)+refreshMinInterval > now {
		return
	}
	atomic.StoreInt64(&lastRefreshTimeStamp, now)
	for _, a := range avs {
		a.UpdateLinkMap()
	}
}

func main() {
	flag.StringVarP(&username, "username", "u", "missdeer", "appveyor username")
	flag.StringVarP(&project, "project", "p", "coredns-custom-build", "appveyor project slug, can be multiple project names separated by semicolon")
	flag.Parse()

	projects = strings.Split(project, ";")

	redis := os.Getenv("REDIS")
	if redis == "" {
		redis = "127.0.0.1:6379"
	}

	if rc = RedisInit(redis); rc == nil {
		return
	}

	for _, p := range projects {
		avs = append(avs, &Appveyor{Username: username, Project: p})
	}

	go updateLinkMap()

	r := gin.Default()
	r.LoadHTMLGlob("templates/*")
	r.NoRoute(func(c *gin.Context) {
		c.Redirect(http.StatusFound, "https://github.com/missdeer/coredns_custom_build")
	})
	r.GET("/dl/*baseName", handler)
	r.GET("/refresh", func(c *gin.Context) {
		updateLinkMap()
		c.JSON(200, gin.H{"result": "OK"})
	})
	r.POST("/refresh", func(c *gin.Context) {
		updateLinkMap()
		c.JSON(200, gin.H{"result": "OK"})
	})

	bind := os.Getenv("BIND")
	if bind == "" {
		bind = ":8765"
	}
	log.Fatal(r.Run(bind))
}
