package metadata

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/MikeO7/kinosail/packages/library"
)

func (client *TMDBClient) downloadCast(ctx context.Context, cast []TMDBCastMember, directory string) []library.Person { //nolint:cyclop,gocognit // Bounded downloads preserve cast order and target naming.
	people := make([]library.Person, 0, min(15, len(cast)))
	paths := make([]string, 0, cap(people))
	for _, actor := range cast {
		if len(people) == 15 {
			break
		}
		if name := strings.TrimSpace(actor.Name); name != "" {
			people = append(people, library.Person{Name: name, Role: strings.TrimSpace(actor.Character)})
			paths = append(paths, actor.ProfilePath)
		}
	}
	jobs := make([]int, 0, len(people))
	for index, path := range paths {
		if path != "" {
			jobs = append(jobs, index)
		}
	}
	workerCount := min(len(jobs), max(1, min(2, runtime.GOMAXPROCS(0)-1)))
	download := func(index int) {
		people[index].Image, _ = client.download(ctx, paths[index], filepath.Join(directory, fmt.Sprintf("person-%d", index)))
	}
	if workerCount < 2 {
		for _, index := range jobs {
			download(index)
		}
		return people
	}
	queue := make(chan int)
	var workers sync.WaitGroup
	for range workerCount {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for index := range queue {
				download(index)
			}
		}()
	}
	for _, index := range jobs {
		queue <- index
	}
	close(queue)
	workers.Wait()
	return people
}
