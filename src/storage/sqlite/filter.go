package sqlite

import (
	"database/sql"
	"log"

	"github.com/nkanaev/yarr/src/storage/model"
)

func (s *SQLiteStorage) ListFilters() []model.Filter {
	result := make([]model.Filter, 0)
	rows, err := s.db.Query(`
		select
			f.id, f.pattern, f.match_content, f.case_sensitive, f.feed_id, f.enabled,
			(select count(*) from items i where i.filter_id = f.id and i.status != :starred)
		from filters f
		order by f.id
	`, sql.Named("starred", model.STARRED))
	if err != nil {
		log.Print(err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var f model.Filter
		err = rows.Scan(
			&f.Id, &f.Pattern, &f.MatchContent, &f.CaseSensitive, &f.FeedId, &f.Enabled,
			&f.Hidden,
		)
		if err != nil {
			log.Print(err)
			return result
		}
		result = append(result, f)
	}
	return result
}

func (s *SQLiteStorage) CreateFilter(params model.FilterParams) *model.Filter {
	var id int64
	err := s.db.QueryRow(`
		insert into filters (pattern, match_content, case_sensitive, feed_id, enabled)
		values (:pattern, :match_content, :case_sensitive, :feed_id, :enabled)
		returning id`,
		sql.Named("pattern", params.Pattern),
		sql.Named("match_content", params.MatchContent),
		sql.Named("case_sensitive", params.CaseSensitive),
		sql.Named("feed_id", params.FeedId),
		sql.Named("enabled", params.Enabled),
	).Scan(&id)
	if err != nil {
		log.Print(err)
		return nil
	}
	return &model.Filter{
		Id:            id,
		Pattern:       params.Pattern,
		MatchContent:  params.MatchContent,
		CaseSensitive: params.CaseSensitive,
		FeedId:        params.FeedId,
		Enabled:       params.Enabled,
	}
}

func (s *SQLiteStorage) UpdateFilter(id int64, params model.FilterParams) bool {
	_, err := s.db.Exec(`
		update filters set
			pattern        = :pattern,
			match_content  = :match_content,
			case_sensitive = :case_sensitive,
			feed_id        = :feed_id,
			enabled        = :enabled
		where id = :id`,
		sql.Named("id", id),
		sql.Named("pattern", params.Pattern),
		sql.Named("match_content", params.MatchContent),
		sql.Named("case_sensitive", params.CaseSensitive),
		sql.Named("feed_id", params.FeedId),
		sql.Named("enabled", params.Enabled),
	)
	if err != nil {
		log.Print(err)
	}
	return err == nil
}

func (s *SQLiteStorage) DeleteFilter(id int64) bool {
	_, err := s.db.Exec(`delete from filters where id = :id`, sql.Named("id", id))
	if err != nil {
		log.Print(err)
	}
	return err == nil
}

func (s *SQLiteStorage) ForEachItem(withContent bool, fn func(item model.Item)) error {
	content := "''"
	if withContent {
		content = "content"
	}
	rows, err := s.db.Query(`
		select id, feed_id, title, date, status, filter_id, ` + content + `
		from items
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var x model.Item
		err = rows.Scan(&x.Id, &x.FeedId, &x.Title, &x.Date, &x.Status, &x.FilterId, &x.Content)
		if err != nil {
			return err
		}
		fn(x)
	}
	return rows.Err()
}

func (s *SQLiteStorage) SetItemFilters(filterIds map[int64]*int64) bool {
	tx, err := s.db.Begin()
	if err != nil {
		log.Print(err)
		return false
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`update items set filter_id = :filter_id where id = :id`)
	if err != nil {
		log.Print(err)
		return false
	}
	defer stmt.Close()

	for id, filterId := range filterIds {
		if _, err := stmt.Exec(sql.Named("id", id), sql.Named("filter_id", filterId)); err != nil {
			log.Print(err)
			return false
		}
	}
	if err := tx.Commit(); err != nil {
		log.Print(err)
		return false
	}
	return true
}
