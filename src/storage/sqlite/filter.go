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
			f.id, f.pattern, f.match_content, f.match_page, f.case_sensitive, f.feed_id, f.enabled,
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
			&f.Id, &f.Pattern, &f.MatchContent, &f.MatchPage, &f.CaseSensitive, &f.FeedId, &f.Enabled,
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
		insert into filters (pattern, match_content, match_page, case_sensitive, feed_id, enabled)
		values (:pattern, :match_content, :match_page, :case_sensitive, :feed_id, :enabled)
		returning id`,
		sql.Named("pattern", params.Pattern),
		sql.Named("match_content", params.MatchContent),
		sql.Named("match_page", params.MatchPage),
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
		MatchPage:     params.MatchPage,
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
			match_page     = :match_page,
			case_sensitive = :case_sensitive,
			feed_id        = :feed_id,
			enabled        = :enabled
		where id = :id`,
		sql.Named("id", id),
		sql.Named("pattern", params.Pattern),
		sql.Named("match_content", params.MatchContent),
		sql.Named("match_page", params.MatchPage),
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

func (s *SQLiteStorage) ForEachItem(withContent, withPage bool, fn func(item model.Item)) error {
	content := "''"
	if withContent {
		content = "i.content"
	}
	page := "null"
	if withPage {
		page = "p.content"
	}
	rows, err := s.db.Query(`
		select i.id, i.feed_id, i.title, i.date, i.status, i.filter_id, ` + content + `, ` + page + `
		from items i
		left join item_pages p on p.item_id = i.id
	`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var x model.Item
		err = rows.Scan(&x.Id, &x.FeedId, &x.Title, &x.Date, &x.Status, &x.FilterId, &x.Content, &x.Page)
		if err != nil {
			return err
		}
		fn(x)
	}
	return rows.Err()
}

func (s *SQLiteStorage) ListItemGUIDs(feedId int64) map[string]bool {
	result := make(map[string]bool)
	rows, err := s.db.Query(`select guid from items where feed_id = :feed_id`, sql.Named("feed_id", feedId))
	if err != nil {
		log.Print(err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var guid string
		if err := rows.Scan(&guid); err != nil {
			log.Print(err)
			return result
		}
		result[guid] = true
	}
	return result
}

// ListItemsWithoutPage returns the items among the newest of a feed whose
// article page has not been fetched yet.
func (s *SQLiteStorage) ListItemsWithoutPage(feedId int64, limit int) []model.Item {
	result := make([]model.Item, 0)
	rows, err := s.db.Query(`
		select i.id, i.feed_id, i.link
		from (
			select id, feed_id, link, date
			from items
			where feed_id = :feed_id
			order by date desc, id desc
			limit :limit
		) i
		where not exists (select 1 from item_pages p where p.item_id = i.id)
		order by i.date desc, i.id desc
	`, sql.Named("feed_id", feedId), sql.Named("limit", limit))
	if err != nil {
		log.Print(err)
		return result
	}
	defer rows.Close()

	for rows.Next() {
		var x model.Item
		if err := rows.Scan(&x.Id, &x.FeedId, &x.Link); err != nil {
			log.Print(err)
			return result
		}
		result = append(result, x)
	}
	return result
}

func (s *SQLiteStorage) SetItemPages(pages map[int64]string) bool {
	tx, err := s.db.Begin()
	if err != nil {
		log.Print(err)
		return false
	}
	defer tx.Rollback()

	for id, content := range pages {
		if err := setItemPage(tx, id, content); err != nil {
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

func setItemPage(tx *sql.Tx, itemId int64, content string) error {
	_, err := tx.Exec(`
		insert into item_pages (item_id, content) values (:item_id, :content)
		on conflict (item_id) do update set content = :content`,
		sql.Named("item_id", itemId),
		sql.Named("content", content),
	)
	return err
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
