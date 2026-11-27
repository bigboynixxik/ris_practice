// Command ris runs the application and its dataset loading commands.
package main

import (
	"context"
	"fmt"
	"os"

	"ris/internal/app"
)

func main() {
	a, err := app.New(os.Args[1:])
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	if err := a.Run(context.Background()); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
