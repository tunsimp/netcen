package main
import (
	"context"
	"log"
	"time"
	mangapb "project/internal/grpc/gen"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main(){
	var conn *grpc.ClientConn
	conn, err := grpc.Dial("localhost:9090", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()

	c := mangapb.NewMangaServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	getMangaResp, err := c.GetManga(ctx, &mangapb.GetMangaRequest{Id: "one-piece"})
	if err != nil {
		log.Fatalf("error when calling GetManga: %v", err)
	}
	log.Printf("GetManga response: %+v \n", getMangaResp)

	searchResp, err := c.SearchManga(ctx, &mangapb.SearchRequest{Keyword: "chain"})
	if err != nil {
		log.Fatalf("error when calling SearchManga: %v", err)
	}
	log.Printf("SearchManga response: %+v \n", searchResp)

	updateResp, err := c.UpdateProgress(ctx, &mangapb.ProgressRequest{UserId: "1", MangaId: "one-piece", CurrentChapter: 1, Status: "reading"})
	if err != nil {
		log.Fatalf("error when calling UpdateProgress: %v", err)
	}
	log.Printf("UpdateProgress response: %+v \n", updateResp)
}
