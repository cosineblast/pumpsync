package main

import (
	"log"
	"os"
	"context"
	"github.com/urfave/cli/v3"
    "the_thing/internal/mediasync"
)

func CliMain() {
	cmd := &cli.Command{
		Name: "pumpsync",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:     "background",
				Aliases:  []string{"bg"},
				Usage:    "Path to the video containing the gameplay",
				Required: true,
			},

			&cli.StringFlag{
				Name:     "link",
				Aliases:  []string{"l"},
				Usage:    "Link to youtube video with the high-quality recording of music you want to overwite with",
				Required: true,
			},

			&cli.StringFlag{
				Name:     "output",
				Aliases:  []string{"o"},
				Usage:    "The location to write the modified background video",
				Required: true,
			},
		},

		Usage: "Overwite audio of an video with music from youtube",

		Action: func(ctx context.Context, cmd *cli.Command) error {

			link := cmd.String("link")
			backgroundPath := cmd.String("background")
			outputPath := cmd.String("output")

			result, err := mediasync.ImproveAudio(backgroundPath, link)

			if err != nil {
				log.Fatal(err)
				os.Exit(1)
			}

            err = os.Rename(result, outputPath)

            if err != nil {
                os.Remove(result)
                return err
            }

			return nil
		},
	}

	if err := cmd.Run(context.Background(), os.Args); err != nil {
		log.Fatal(err)
		os.Exit(1)
	}
}
