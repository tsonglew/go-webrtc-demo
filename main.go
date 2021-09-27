package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pion/webrtc/v3"
)

var pc *webrtc.PeerConnection
var dc *webrtc.DataChannel

type OfferData struct {
	SDP  string `json:"sdp"`
	Type string `json:"type"`
}
type TimeMsg struct {
	Now int64 `json:"now"`
}

func main() {
	r := gin.Default()
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "pong",
		})
	})
	r.GET("/", index)
	r.GET("/client.js", javascript)
	r.POST("/offer", offer)
	r.Run() // listen and serve on
}

func index(c *gin.Context) {
	data, err := os.ReadFile("index.html")
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

func javascript(c *gin.Context) {
	data, err := os.ReadFile("client.js")
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", []byte(data))
}

func serve(pc *webrtc.PeerConnection, offer webrtc.SessionDescription) {
	log.Print("Serve")
	pc.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
	})
	pc.OnTrack(func(tr *webrtc.TrackRemote, r *webrtc.RTPReceiver) {
		log.Printf("Track received %s \n", tr.Kind().String())
		for {
			_, _, err := tr.ReadRTP()
			if err != nil {
				fmt.Printf("Error reading RTP: %v", err)
				continue
			}
			if dc != nil {
				js, _ := json.Marshal(TimeMsg{Now: time.Now().UnixNano() / int64(time.Millisecond)})
				senderr := dc.SendText(string(js))
				if senderr != nil {
					panic(senderr)
				}
			}
		}
	})
	pc.OnDataChannel(func(dchan *webrtc.DataChannel) {
		log.Printf("DataChannel received %s \n", dchan.Label())
		dchan.OnOpen(func() {
			fmt.Printf("DataChannel opened \n")
			dc = dchan
		})

		// dc.OnMessage(func(msg webrtc.DataChannelMessage) {
		// 	fmt.Printf("%s\n", string(msg.Data))
		// })
	})

	err := pc.SetRemoteDescription(offer)
	if err != nil {
		panic(err)
	}
	answer, err := pc.CreateAnswer(nil)
	if err != nil {
		panic(err)
	}

	gatherComplete := webrtc.GatheringCompletePromise(pc)
	err = pc.SetLocalDescription(answer)
	if err != nil {
		panic(err)
	}

	<-gatherComplete
}

func offer(c *gin.Context) {
	var offerData OfferData
	c.BindJSON(&offerData)
	of := webrtc.SessionDescription{SDP: offerData.SDP, Type: webrtc.NewSDPType(offerData.Type)}
	var err error
	pc, err = webrtc.NewPeerConnection(webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	})
	if err != nil {
		c.String(http.StatusInternalServerError, "Internal Server Error")
		return
	}
	// defer func() {
	// 	if err = pc.Close(); err != nil {
	// 		c.String(http.StatusInternalServerError, "Internal Server Error")
	// 	}
	// }()

	serve(pc, of)
	c.JSON(http.StatusOK, gin.H{"sdp": pc.LocalDescription().SDP, "type": pc.LocalDescription().Type.String()})
}
