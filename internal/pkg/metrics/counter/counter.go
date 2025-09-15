package counter

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ManuelReschke/PixelFox/internal/pkg/cache"
	"github.com/ManuelReschke/PixelFox/internal/pkg/database"
)

const (
	imageViewsKey     = "image:counters:views"
	imageDownloadsKey = "image:counters:downloads"
	albumViewsKey     = "album:counters:views"
)

// AddImageView increments the pending view counter for an image in Redis
func AddImageView(imageID uint) error {
	ctx := context.Background()
	field := strconv.FormatUint(uint64(imageID), 10)
	return cache.GetClient().HIncrBy(ctx, imageViewsKey, field, 1).Err()
}

// AddImageDownload increments the pending download counter for an image in Redis
func AddImageDownload(imageID uint) error {
	ctx := context.Background()
	field := strconv.FormatUint(uint64(imageID), 10)
	return cache.GetClient().HIncrBy(ctx, imageDownloadsKey, field, 1).Err()
}

// FlushAll flushes both views and downloads to the database
func FlushAll() error {
	if err := flushHashToTable(imageViewsKey, "images", "view_count"); err != nil {
		return err
	}
	if err := flushHashToTable(imageDownloadsKey, "images", "download_count"); err != nil {
		return err
	}
	if err := flushHashToTable(albumViewsKey, "albums", "view_count"); err != nil {
		return err
	}
	return nil
}

// flushHashToTable drains a Redis hash atomically and applies batched increments to images table.
// Uses RENAME to a temporary key for atomic drain without losing in-flight increments.
func flushHashToTable(redisKey, table, column string) error {
	ctx := context.Background()
	rdb := cache.GetClient()

	// Atomically move the hash to a temp key for draining
	tmpKey := fmt.Sprintf("%s:tmp:%d", redisKey, time.Now().UnixNano())
	if err := rdb.Do(ctx, "RENAME", redisKey, tmpKey).Err(); err != nil {
		// If key does not exist, nothing to flush
		if strings.Contains(err.Error(), "no such key") || strings.Contains(strings.ToLower(err.Error()), "no such key") {
			return nil
		}
		// Some Redis libs return redis.Nil; treat as empty
		if err.Error() == "redis: nil" {
			return nil
		}
		return err
	}

	// Ensure cleanup of tmpKey even if later steps fail
	defer rdb.Del(ctx, tmpKey)

	data, err := rdb.HGetAll(ctx, tmpKey).Result()
	if err != nil {
		return err
	}
	if len(data) == 0 {
		return nil
	}

	// Build batched UPDATE using CASE WHEN id THEN inc
	// Collect ids and increments; also sort ids for stable SQL
	type pair struct {
		id  uint64
		inc int64
	}
	pairs := make([]pair, 0, len(data))
	for k, v := range data {
		id, perr := strconv.ParseUint(k, 10, 64)
		if perr != nil {
			continue
		}
		inc, ierr := strconv.ParseInt(v, 10, 64)
		if ierr != nil || inc == 0 {
			continue
		}
		pairs = append(pairs, pair{id: id, inc: inc})
	}
	if len(pairs) == 0 {
		return nil
	}
	sort.Slice(pairs, func(i, j int) bool { return pairs[i].id < pairs[j].id })

	// Compose SQL
	// UPDATE images SET <column> = <column> + CASE id WHEN ? THEN ? ... END WHERE id IN ( ... )
	var builder strings.Builder
	args := make([]interface{}, 0, len(pairs)*2+len(pairs))
	builder.WriteString("UPDATE ")
	builder.WriteString(table)
	builder.WriteString(" SET ")
	builder.WriteString(column)
	builder.WriteString(" = ")
	builder.WriteString(column)
	builder.WriteString(" + CASE id ")
	for _, p := range pairs {
		builder.WriteString(" WHEN ? THEN ?")
		args = append(args, p.id, p.inc)
	}
	builder.WriteString(" END WHERE id IN (")
	for i, p := range pairs {
		if i > 0 {
			builder.WriteString(",")
		}
		builder.WriteString("?")
		args = append(args, p.id)
	}
	builder.WriteString(")")

	sql := builder.String()
	db := database.GetDB()
	if err := db.Exec(sql, args...).Error; err != nil {
		return err
	}
	return nil
}

// AddAlbumView increments the pending view counter for an album in Redis
func AddAlbumView(albumID uint) error {
	ctx := context.Background()
	field := strconv.FormatUint(uint64(albumID), 10)
	return cache.GetClient().HIncrBy(ctx, albumViewsKey, field, 1).Err()
}
