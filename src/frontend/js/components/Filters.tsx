import { useSignal, useComputed } from "@preact/signals";
import { useEffect, useMemo, useRef } from "preact/hooks";
import type { ComponentChildren } from "preact";
import api, { HTTPError } from "../api";
import { useI18n } from "../i18n";
import { debounce, to } from "../utils";
import Icon from "./Icon";
import RelativeTime from "./RelativeTime";
import Tooltip from "./Tooltip";
import type {
  Feed,
  Folder,
  Filter,
  FilterParams,
  FilterPreviewMatch,
  FilterPreviewResponse,
} from "../api-types";

const PREVIEW_DELAY = 300;
// asking again for the preview while article pages are fetched
const PAGES_POLL_DELAY = 2000;

type Field = "title" | "content" | "page";

const EXAMPLES = [
  { pattern: "sponsored|advertisement", content: false },
  { pattern: "^\\[(ad|promo)\\]", content: false },
  { pattern: "\\bgiveaway\\b", content: true },
];

type Selected = number | "new" | null;

interface Preview {
  loading: boolean;
  error: string;
  count: number;
  list: FilterPreviewMatch[];
  pages?: FilterPreviewResponse["pages"];
}

const EMPTY_PREVIEW: Preview = { loading: false, error: "", count: 0, list: [] };

function emptyForm(): FilterParams {
  return {
    pattern: "",
    match_content: false,
    match_page: false,
    case_sensitive: false,
    feed_id: null,
    enabled: true,
  };
}

function sameForm(a: FilterParams, b: FilterParams): boolean {
  return (
    a.pattern === b.pattern &&
    a.match_content === b.match_content &&
    a.match_page === b.match_page &&
    a.case_sensitive === b.case_sensitive &&
    a.feed_id === b.feed_id &&
    a.enabled === b.enabled
  );
}

function fieldOf(f: Pick<FilterParams, "match_content" | "match_page">): Field {
  if (f.match_page) return "page";
  return f.match_content ? "content" : "title";
}

function regexLiteral(f: Pick<FilterParams, "pattern" | "case_sensitive">): string {
  return "/" + f.pattern + "/" + (f.case_sensitive ? "" : "i");
}

interface FiltersProps {
  filters: Filter[];
  feeds: Feed[];
  folders: Folder[];
  feedsById: Record<number, Feed>;
  listWidth: number;
  language: string;
  // a new filter to start editing, e.g. from "filter similar articles"
  draft: Partial<FilterParams> | null;
  onClose: () => void;
  onChanged: () => void;
  // resize handle, shared with the item list
  listHandle?: ComponentChildren;
}

export default function Filters({
  filters,
  feeds,
  folders,
  feedsById,
  listWidth,
  language,
  draft,
  onClose,
  onChanged,
  listHandle,
}: FiltersProps) {
  const { t } = useI18n();

  const selected = useSignal<Selected>(null);
  const form = useSignal<FilterParams>(emptyForm());
  const saving = useSignal(false);
  const refetching = useSignal(false);
  const saveError = useSignal("");
  const preview = useSignal<Preview>(EMPTY_PREVIEW);
  const previewSeq = useRef(0);
  const pagesPoll = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const patternRef = useRef<HTMLInputElement>(null);

  const original = (): FilterParams | null => {
    if (selected.value === "new") return emptyForm();
    const f = filters.find(f => f.id === selected.value);
    if (!f) return null;
    const { pattern, match_content, match_page, case_sensitive, feed_id, enabled } = f;
    return { pattern, match_content, match_page, case_sensitive, feed_id, enabled };
  };
  // pages are fetched only for the feeds that need them
  const pageNeedsFeed = form.value.match_page && form.value.feed_id === null;
  const orig = original();
  const dirty = !!orig && !sameForm(form.value, orig);
  const canSave =
    !saving.value &&
    !!form.value.pattern.trim() &&
    !preview.value.error &&
    !pageNeedsFeed &&
    (selected.value === "new" || dirty);

  const totalHidden = filters.reduce((sum, f) => sum + (f.enabled ? f.hidden : 0), 0);

  const feedGroups = useMemo(() => {
    const byFolder = new Map<number | null, Feed[]>();
    for (const feed of feeds) {
      if (!byFolder.has(feed.folder_id)) byFolder.set(feed.folder_id, []);
      byFolder.get(feed.folder_id)!.push(feed);
    }
    const groups = folders
      .filter(folder => byFolder.has(folder.id))
      .map(folder => ({ title: folder.title, feeds: byFolder.get(folder.id)! }));
    if (byFolder.has(null)) groups.push({ title: "──", feeds: byFolder.get(null)! });
    return groups;
  }, [feeds, folders]);

  const patternEmpty = useComputed(() => !form.value.pattern.trim());

  const runPreview = async () => {
    const f = form.value;
    if (!f.pattern.trim() || (f.match_page && f.feed_id === null)) return;
    const seq = ++previewSeq.current;
    const [err, data] = await to(api.filters.preview({ ...form.value, enabled: true }));
    if (seq !== previewSeq.current) return; // the form changed in the meantime
    if (err) {
      preview.value = EMPTY_PREVIEW;
      return;
    }
    const wasFetching = !!preview.value.pages?.fetching;
    preview.value = {
      loading: false,
      error: data.error || "",
      count: data.count,
      list: data.list,
      pages: data.pages,
    };
    if (data.pages?.fetching) {
      pagesPoll.current = setTimeout(() => runPreviewRef.current(), PAGES_POLL_DELAY);
    } else if (wasFetching) {
      // the fetched pages may have changed what saved filters hide
      onChanged();
    }
  };
  const runPreviewRef = useRef(runPreview);
  runPreviewRef.current = runPreview;
  const schedulePreview = useMemo(() => debounce(() => runPreviewRef.current(), PREVIEW_DELAY), []);

  // a changed form invalidates the save error and the preview
  useEffect(() => {
    saveError.value = "";
    clearTimeout(pagesPoll.current);
    if (!form.value.pattern.trim() || pageNeedsFeed) {
      previewSeq.current++;
      preview.value = EMPTY_PREVIEW;
      return;
    }
    preview.value = { ...preview.value, loading: true };
    schedulePreview();
  }, [form.value]);

  useEffect(() => () => clearTimeout(pagesPoll.current), []);

  const fieldLabel = (f: Pick<FilterParams, "match_content" | "match_page">): string => {
    const field = fieldOf(f);
    if (field === "page") return t("filter_field_page");
    return field === "content" ? t("filter_field_content") : t("filter_field_title");
  };

  const setField = (field: Field) =>
    updateForm({ match_content: field === "content", match_page: field === "page" });

  const updateForm = (partial: Partial<FilterParams>) => {
    form.value = { ...form.value, ...partial };
  };

  const focusPattern = () => setTimeout(() => patternRef.current?.focus(), 0);

  const newFilter = (d: Partial<FilterParams> = {}) => {
    selected.value = "new";
    form.value = { ...emptyForm(), ...d };
    focusPattern();
  };

  useEffect(() => {
    if (draft) newFilter(draft);
  }, [draft]);

  const revert = () => {
    const o = original();
    if (o) form.value = { ...o };
  };

  const selectFilter = (f: Filter) => {
    selected.value = f.id;
    revert();
  };

  const scopeTitle = (feedId: number | null): string => {
    if (feedId === null) return t("all_feeds");
    return feedsById[feedId]?.title || "";
  };

  const save = async () => {
    if (!canSave) return;
    saving.value = true;
    const data = { ...form.value, pattern: form.value.pattern.trim() };
    const [err, saved] = await to(
      selected.value === "new"
        ? api.filters.create(data)
        : api.filters.update(selected.value as number, data),
    );
    saving.value = false;
    if (err) {
      saveError.value =
        t("fail_save_filter") +
        (err instanceof HTTPError ? ` (${err.status} ${err.statusText})` : "");
      return;
    }
    updateForm({ pattern: data.pattern });
    onChanged();
    selected.value = saved.id;
  };

  const refetch = async () => {
    if (typeof selected.value !== "number") return;
    refetching.value = true;
    const [err] = await to(api.filters.refetch(selected.value));
    refetching.value = false;
    // already fetching is fine, the preview shows the progress either way
    if (err && !(err instanceof HTTPError && err.status === 409)) {
      saveError.value =
        t("fail_refetch") + (err instanceof HTTPError ? ` (${err.status} ${err.statusText})` : "");
      return;
    }
    clearTimeout(pagesPoll.current);
    runPreview();
  };

  const deleteFilter = async () => {
    const f = filters.find(f => f.id === selected.value);
    if (!f || !confirm(t("confirm_delete", { name: regexLiteral(f) }))) return;
    const [err] = await to(api.filters.delete(f.id));
    if (err) {
      saveError.value = t("fail_save_filter");
      return;
    }
    selected.value = null;
    onChanged();
  };

  const p = preview.value;

  return (
    <div
      id="col-filters"
      style={{ minWidth: 0 }}
      className={`vh-100 d-flex flex-grow-1 ${selected.value !== null ? "is-editing" : ""}`}>
      {/* filter list */}
      <div
        className="filters-list position-relative d-flex flex-column border-end flex-shrink-0"
        style={{ width: `${listWidth}px` }}>
        {listHandle}
        <div className="px-2 py-1 d-flex gap-1 align-items-center">
          <Tooltip label={t("show_feeds")}>
            <button
              type="button"
              className="c-button-pill d-lg-none"
              aria-label={t("show_feeds")}
              onClick={onClose}>
              <Icon name="chevron-left" />
            </button>
          </Tooltip>
          <div className="flex-grow-1 px-1 text-truncate user-select-none fw-semibold">
            {t("filters")}
          </div>
          {totalHidden > 0 && (
            <Tooltip label={t("filters_hidden", { count: totalHidden })} className="flex-shrink-0">
              <span className="d-flex align-items-center gap-1 px-1 small opacity-75 user-select-none">
                <Icon name="eye-off" />
                {totalHidden}
              </span>
            </Tooltip>
          )}
          <Tooltip label={t("new_filter")}>
            <button
              type="button"
              className="c-button-pill"
              aria-label={t("new_filter")}
              aria-pressed={selected.value === "new"}
              onClick={() => newFilter()}>
              <Icon name="plus" />
            </button>
          </Tooltip>
        </div>
        <div className="d-flex flex-column p-2 overflow-auto border-top flex-grow-1 gap-1">
          {filters.map(f => (
            <div
              key={f.id}
              className={`c-listitem d-flex flex-column gap-1 user-select-none ${
                f.enabled ? "" : "is-read"
              }`}
              role="radio"
              aria-checked={selected.value === f.id}
              onClick={() => selectFilter(f)}>
              <code className="filter-pattern text-truncate">{regexLiteral(f)}</code>
              <small className="d-flex align-items-center gap-1 opacity-75 min-w-0">
                {f.feed_id && feedsById[f.feed_id]?.icon ? (
                  <span className="c-icon">
                    <img src={feedsById[f.feed_id].icon!} alt="" loading="lazy" />
                  </span>
                ) : (
                  <Icon name={f.feed_id ? "rss" : "layers"} />
                )}
                <span className="text-truncate">{scopeTitle(f.feed_id)}</span>
                <span className="flex-shrink-0">·</span>
                <span className="flex-shrink-0">{fieldLabel(f)}</span>
                <span className="flex-grow-1" />
                {!f.enabled ? (
                  <span className="flex-shrink-0">{t("filter_paused")}</span>
                ) : (
                  <Tooltip
                    label={t("filters_hidden", { count: f.hidden })}
                    className="flex-shrink-0">
                    <span className="d-flex align-items-center gap-1">
                      <Icon name="eye-off" />
                      {f.hidden}
                    </span>
                  </Tooltip>
                )}
              </small>
            </div>
          ))}
        </div>
      </div>

      {/* filter editor */}
      <div className="filters-editor d-flex flex-column flex-grow-1" style={{ minWidth: 0 }}>
        <div className="px-2 py-1 d-flex gap-1 align-items-center">
          <Tooltip label={t("filters")}>
            <button
              type="button"
              className="c-button-pill d-lg-none"
              aria-label={t("filters")}
              onClick={() => (selected.value = null)}>
              <Icon name="chevron-left" />
            </button>
          </Tooltip>
          <div className="flex-grow-1 px-1 text-truncate user-select-none fw-semibold">
            {selected.value === "new"
              ? t("new_filter")
              : selected.value !== null
                ? t("edit_filter")
                : ""}
          </div>
          {typeof selected.value === "number" && (
            <Tooltip label={t("delete")}>
              <button
                type="button"
                className="c-button-pill text-danger"
                aria-label={t("delete")}
                onClick={deleteFilter}>
                <Icon name="trash" />
              </button>
            </Tooltip>
          )}
          <Tooltip label={t("close")}>
            <button
              type="button"
              className="c-button-pill"
              aria-label={t("close")}
              onClick={onClose}>
              <Icon name="x" />
            </button>
          </Tooltip>
        </div>

        <div className="overflow-auto border-top flex-grow-1">
          {selected.value === null ? (
            /* empty state */
            <div className="filters-intro mx-auto px-4 py-5">
              <div className="filters-intro-icon mb-3">
                <Icon name="filter" />
              </div>
              <h2 className="mb-2" style={{ fontSize: "1.25rem" }}>
                {t("filter_empty_title")}
              </h2>
              <p className="opacity-75">{t("filter_empty_text")}</p>
              <p className="small opacity-75 mb-2">{t("filter_examples")}</p>
              <div className="d-flex flex-wrap gap-2">
                {EXAMPLES.map(example => (
                  <button
                    key={example.pattern}
                    type="button"
                    className="c-button filter-example"
                    onClick={() =>
                      newFilter({ pattern: example.pattern, match_content: example.content })
                    }>
                    <code>{regexLiteral({ pattern: example.pattern, case_sensitive: false })}</code>
                  </button>
                ))}
              </div>
            </div>
          ) : (
            /* form & live preview */
            <form
              className="filters-form mx-auto px-4 py-3"
              onSubmit={e => {
                e.preventDefault();
                save();
              }}>
              <div className="filter-sentence d-flex flex-wrap align-items-center gap-2 mb-2">
                <span>{t("filter_hide_when")}</span>
                <div className="c-segmented" role="group">
                  <button
                    type="button"
                    aria-pressed={fieldOf(form.value) === "title"}
                    onClick={() => setField("title")}>
                    {t("filter_field_title")}
                  </button>
                  <button
                    type="button"
                    aria-pressed={fieldOf(form.value) === "content"}
                    onClick={() => setField("content")}>
                    {t("filter_field_content")}
                  </button>
                  <button
                    type="button"
                    aria-pressed={fieldOf(form.value) === "page"}
                    onClick={() => setField("page")}>
                    {t("filter_field_page")}
                  </button>
                </div>
                <span>{t("filter_matches")}</span>
              </div>

              <div
                className={`filter-regex d-flex align-items-center ${p.error ? "is-invalid" : ""}`}>
                <span className="filter-regex-slash">/</span>
                <input
                  className="flex-grow-1"
                  ref={patternRef}
                  value={form.value.pattern}
                  onInput={e => updateForm({ pattern: (e.target as HTMLInputElement).value })}
                  spellcheck={false}
                  autocomplete="off"
                  autocapitalize="off"
                  aria-invalid={!!p.error}
                  aria-describedby="filter-pattern-hint"
                />
                <span className="filter-regex-slash">/{form.value.case_sensitive ? "" : "i"}</span>
                <Tooltip label={t("filter_case_sensitive")}>
                  <button
                    type="button"
                    className="c-button-pill filter-case"
                    aria-label={t("filter_case_sensitive")}
                    aria-pressed={form.value.case_sensitive}
                    onClick={() => updateForm({ case_sensitive: !form.value.case_sensitive })}>
                    Aa
                  </button>
                </Tooltip>
              </div>
              <div id="filter-pattern-hint" className="small mt-1 mb-3">
                {p.error ? (
                  <span className="text-danger">{p.error}</span>
                ) : (
                  <span className="opacity-75">{t("filter_syntax_hint")}</span>
                )}
              </div>

              <div
                className={`d-flex flex-wrap align-items-center gap-2 ${
                  form.value.match_page ? "mb-1" : "mb-3"
                }`}>
                <label htmlFor="filter-scope">{t("filter_in")}</label>
                <select
                  id="filter-scope"
                  className="c-input flex-grow-1"
                  style={{ minWidth: 0 }}
                  value={form.value.feed_id === null ? "" : String(form.value.feed_id)}
                  onChange={e => {
                    const v = (e.target as HTMLSelectElement).value;
                    updateForm({ feed_id: v === "" ? null : Number(v) });
                  }}>
                  <option value="">{t("all_feeds")}</option>
                  {feedGroups.map(group => (
                    <optgroup key={group.title} label={group.title}>
                      {group.feeds.map(feed => (
                        <option key={feed.id} value={String(feed.id)}>
                          {feed.title}
                        </option>
                      ))}
                    </optgroup>
                  ))}
                </select>
              </div>
              {form.value.match_page && (
                <div className="small mb-3">
                  {pageNeedsFeed ? (
                    <span className="text-danger">{t("filter_page_needs_feed")}</span>
                  ) : (
                    <span className="opacity-75">{t("filter_page_hint")}</span>
                  )}
                </div>
              )}

              <div className="d-flex flex-wrap align-items-center gap-2">
                <label className="d-flex align-items-center gap-2 cursor-pointer user-select-none me-auto">
                  <input
                    type="checkbox"
                    checked={form.value.enabled}
                    onChange={e => updateForm({ enabled: (e.target as HTMLInputElement).checked })}
                  />
                  {t("filter_enabled")}
                </label>
                {dirty && selected.value !== "new" && (
                  <button type="button" className="c-button" onClick={revert}>
                    {t("revert")}
                  </button>
                )}
                {orig?.match_page && selected.value !== "new" && (
                  <Tooltip label={t("filter_refetch_hint")}>
                    <button
                      type="button"
                      className="c-button"
                      disabled={dirty || refetching.value || !!p.pages?.fetching}
                      onClick={refetch}>
                      {t("filter_refetch")}
                    </button>
                  </Tooltip>
                )}
                <button type="submit" className="c-button c-button-primary" disabled={!canSave}>
                  {t("save")}
                </button>
              </div>
              {saveError.value && <div className="text-danger small mt-2">{saveError.value}</div>}

              <hr />

              {!patternEmpty.value && !p.error && (
                <div className="d-flex align-items-baseline gap-2 mb-2 user-select-none">
                  <span className="fw-semibold">{t("filter_matching", { count: p.count })}</span>
                  {p.count > p.list.length && (
                    <span className="small opacity-75">
                      {t("filter_preview_limit", { count: p.list.length })}
                    </span>
                  )}
                  {p.loading && <span className="c-spinner ms-auto" />}
                </div>
              )}
              {p.pages?.fetching && !p.error && (
                <div className="d-flex align-items-center gap-2 small opacity-75 mb-2 user-select-none">
                  <span className="c-spinner" />
                  {t("filter_pages_fetching", { count: p.pages.left })}
                </div>
              )}
              {!patternEmpty.value && !p.loading && !p.error && !p.count && !p.pages?.fetching && (
                <p className="opacity-75">{t("filter_no_matches")}</p>
              )}
              <div className={`d-flex flex-column gap-1 ${p.loading ? "opacity-50" : ""}`}>
                {p.list.map(m => (
                  <div
                    key={m.id}
                    className={`filter-match px-2 py-1 ${m.status === "starred" ? "is-starred" : ""}`}
                    title={m.status === "starred" ? t("filter_starred_visible") : undefined}>
                    <div className="d-flex align-items-center small opacity-75 gap-1">
                      {m.status === "starred" && <Icon name="star-full" />}
                      <span className="flex-fill text-truncate">{feedsById[m.feed_id]?.title}</span>
                      <span className="flex-shrink-0">
                        <RelativeTime val={m.date} locale={language} />
                      </span>
                    </div>
                    {m.field === "title" ? (
                      <div className="text-break">
                        {m.before}
                        <mark>{m.match}</mark>
                        {m.after}
                      </div>
                    ) : (
                      <>
                        <div className="text-break">{m.title || t("untitled")}</div>
                        <div className="text-break small opacity-75">
                          {m.before}
                          <mark>{m.match}</mark>
                          {m.after}
                        </div>
                      </>
                    )}
                  </div>
                ))}
              </div>
            </form>
          )}
        </div>
      </div>
    </div>
  );
}
