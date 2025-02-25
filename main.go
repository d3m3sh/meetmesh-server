package main

import (
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/pion/webrtc/v3"
)

type Member struct {
	Id   string `json:"id"`
	Name string `json:"name"`
}

type Room struct {
	Id       string   `json:"id"`
	Members  []Member `json:"members"`
	Password string   `json:"password,omitempty"`
}

type EventSSE struct {
	Type    string
	From    Member
	To      string
	Payload any
}

var (
	clientChannels = make(map[string]chan EventSSE) // Map pour gérer les connexions des clients
	rooms          = make(map[string]*Room)
	mutex          sync.Mutex
)

func generateRoomID() string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 6)
	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}
	return string(b)
}

type RequestCreate struct {
	Name     string `json:"name"`
	Password string `json:"password,omitempty"`
}

func createRoom(c *gin.Context) {
	mutex.Lock()
	defer func() {
		mutex.Unlock()
	}()

	var request RequestCreate
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	roomID := generateRoomID()
	room := &Room{Id: roomID, Members: []Member{}, Password: request.Password}
	rooms[roomID] = room
	newMember := Member{
		Id:   generateRoomID(),
		Name: request.Name,
	}
	room.Members = append(room.Members, newMember)
	fmt.Println(room)
	c.JSON(http.StatusOK, gin.H{"room": rooms[roomID], "id": newMember.Id})
}

type RequestJoin struct {
	RoomID   string `json:"room_id"`
	Name     string `json:"name"`
	Password string `json:"password,omitempty"`
}

func joinRoom(c *gin.Context) {
	mutex.Lock()
	defer func() {
		mutex.Unlock()
	}()

	var request RequestJoin
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	if request.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "name is missing"})
		return
	}

	fmt.Println("room_id:", request.RoomID)
	if room, exists := rooms[request.RoomID]; exists {
		newMember := Member{
			Id:   generateRoomID(),
			Name: request.Name,
		}
		if room.Password != request.Password {
			c.JSON(http.StatusForbidden, gin.H{"error": "Access refused"})
			return
		}
		room.Members = append(room.Members, newMember)
		c.JSON(http.StatusOK, gin.H{"room": room, "id": newMember.Id})
	} else {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
	}
}

func streamRoom(c *gin.Context) {
	fmt.Println("streamRoom")
	roomID := c.Param("room_id")
	participantID := c.Param("participant_id")

	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	mutex.Lock()
	room, exists := rooms[roomID]

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		mutex.Unlock()
		return
	}

	var member *Member
	for _, m := range room.Members {
		if m.Id == participantID {
			member = &m
		}
	}

	if member == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request"})
		mutex.Unlock()
		return
	}
	clientChannel := make(chan EventSSE, 10)
	clientChannels[member.Id] = clientChannel
	clientGone := c.Writer.CloseNotify()

	go func() {
		defer func() {
			mutex.Lock()
			defer mutex.Unlock()
			fmt.Println("clean client channel to member id:", member.Id)

			close(clientChannel)
			fmt.Println("client channel is closed to member id:", member.Id)

			delete(clientChannels, member.Id)
			fmt.Println("client channel has deleted to member id:", member.Id)
			for _, r := range rooms {
				for y, m := range r.Members {
					if m.Id == member.Id {
						fmt.Println("client %s delete to room %s\n", member.Id, r.Id)
						r.Members = append(r.Members[:y], r.Members[y+1:]...)
						return
					}
				}
			}
		}()

		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				fmt.Printf("client %s is alive\n", participantID)
				continue
			case <-clientGone:
				fmt.Printf("client %s gone\n", participantID)
				return
			}
		}
	}()

	mutex.Unlock()

	c.Stream(func(w io.Writer) bool {
		if msg, ok := <-clientChannel; ok {
			fmt.Println("msg to send with SSE:", msg.Type)
			c.SSEvent(msg.Type, gin.H{
				"from":    msg.From,
				"payload": msg.Payload,
			})
			return true
		}
		return false
	})
}

type RequestSDP struct {
	SDP  webrtc.SessionDescription `json:"sdp"`
	From string                    `json:"from"`
}

func transferSDP(c *gin.Context) {
	roomID := c.Param("room_id")
	participantID := c.Param("participant_id")

	mutex.Lock()
	defer mutex.Unlock()

	room, exists := rooms[roomID]

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "Room not found"})
		return
	}

	var request RequestSDP
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
		return
	}

	fmt.Printf("Received SDP from %s in room %s\n", participantID, roomID)

	var from *Member
	for _, member := range room.Members {
		if member.Id == request.From {
			from = &member
			break
		}
	}

	if from == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid requester"})
	}

	c.JSON(http.StatusOK, gin.H{"message": "SDP received"})

	for _, member := range room.Members {
		if member.Id == participantID {
			fmt.Println("member found:", member)
			clientChannel, exists := clientChannels[member.Id]
			if exists {
				fmt.Println("channel found:", member)

				clientChannel <- EventSSE{
					Type:    request.SDP.Type.String(),
					From:    *from,
					Payload: request.SDP,
				}

			}
		}
	}
}

func RoomWithActiveMembers(room *Room) Room {
	members := []Member{}
	for _, member := range room.Members {
		if _, found := clientChannels[member.Id]; found {
			members = append(members, member)
		}
	}

	return Room{
		Id:       room.Id,
		Password: room.Password,
		Members:  members,
	}
}

func cleanRooms() {

	for {
		indexRoomEmpty := []string{}
		<-time.After(1 * time.Minute)
		fmt.Printf("checking rooms activity\n")
		mutex.Lock()

		for _, room := range rooms {
			indexMemberGone := []int{}

			fmt.Printf("checking room id %s\n", room.Id)
			for idxMember, member := range room.Members {
				fmt.Printf("\tchecking member %s\n", member.Id)
				//check clientChannel is present
				if _, found := clientChannels[member.Id]; !found {
					fmt.Printf("\t add idx %d to delete\n", idxMember)
					indexMemberGone = append(indexMemberGone, idxMember)
				}
			}

			for i, idx := range indexMemberGone {
				member := room.Members[idx-i]
				fmt.Printf("\t\tdelete member %s %s\n", member.Id, member.Name)
				room.Members = append(room.Members[:idx-i], room.Members[idx-i+1:]...)
				fmt.Printf("\t\tmember %s %s deleted\n", member.Id, member.Name)
			}
			fmt.Printf("\t\troom has now %d members\n", len(room.Members))

			if len(room.Members) == 0 {
				fmt.Printf("\t room id %s is empty\n", room.Id)
				indexRoomEmpty = append(indexRoomEmpty, room.Id)
			}
		}

		fmt.Printf("%d room(s) will be delete\n", len(indexRoomEmpty))
		for _, id := range indexRoomEmpty {
			fmt.Printf("delete room id %s\n", id)
			delete(rooms, id)
			fmt.Printf("room id %s deleted\n", id)
		}
		mutex.Unlock()
	}

}
func main() {
	r := gin.Default()

	corsConfig := cors.Config{
		AllowAllOrigins: true,
		AllowMethods:    []string{"GET", "POST", "PUT", "DELETE"},
		AllowHeaders:    []string{"Origin", "Content-Type", "Authorization"},
	}

	go cleanRooms()

	r.Use(cors.New(corsConfig))
	r.POST("/api/create-room", createRoom)
	r.POST("/api/join-room", joinRoom)
	r.GET("/stream/:room_id/:participant_id", streamRoom)
	r.POST("/api/room/:room_id/:participant_id", transferSDP)

	// r.RunTLS(":8080", "./ssl/meetmesh.crt", "./ssl/meetmesh-encrypt.key")
	r.Run(":8080")
}
