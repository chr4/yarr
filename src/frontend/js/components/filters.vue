<template>
  <div
    id="col-filters"
    class="vh-100 d-flex flex-grow-1"
    style="min-width: 0"
    :class="{ 'is-editing': selected !== null }">
    <!-- filter list -->
    <div
      class="filters-list position-relative d-flex flex-column border-end flex-shrink-0"
      :style="{ width: listWidth + 'px' }">
      <!-- resize handle, shared with the item list -->
      <slot name="list-handle"></slot>
      <div class="px-2 py-1 d-flex gap-1 align-items-center">
        <button class="c-button-pill d-lg-none" @click="$emit('close')" :title="$t('show_feeds')">
          <v-icon name="chevron-left" />
        </button>
        <div class="flex-grow-1 px-1 text-truncate user-select-none fw-semibold">
          {{ $t("filters") }}
        </div>
        <span
          class="flex-shrink-0 d-flex align-items-center gap-1 px-1 small opacity-75 user-select-none"
          :title="$t('filters_hidden', { count: totalHidden })"
          v-if="totalHidden">
          <v-icon name="eye-off" />{{ totalHidden }}
        </span>
        <button
          class="c-button-pill"
          :aria-pressed="selected === 'new'"
          :title="$t('new_filter')"
          @click="newFilter()">
          <v-icon name="plus" />
        </button>
      </div>
      <div class="d-flex flex-column p-2 overflow-auto border-top flex-grow-1 gap-1">
        <div
          v-for="f in filters"
          :key="f.id"
          class="c-listitem d-flex flex-column gap-1 user-select-none"
          :class="{ 'is-read': !f.enabled }"
          role="radio"
          :aria-checked="selected === f.id"
          @click="selectFilter(f)">
          <code class="filter-pattern text-truncate">{{ regexLiteral(f) }}</code>
          <small class="d-flex align-items-center gap-1 opacity-75 min-w-0">
            <span class="c-icon" v-if="f.feed_id && feedsById[f.feed_id]?.icon">
              <img :src="feedsById[f.feed_id].icon || undefined" alt="" loading="lazy" />
            </span>
            <v-icon name="rss" v-else-if="f.feed_id" />
            <v-icon name="layers" v-else />
            <span class="text-truncate">{{ scopeTitle(f.feed_id) }}</span>
            <span class="flex-shrink-0">·</span>
            <span class="flex-shrink-0">{{ fieldLabel(f) }}</span>
            <span class="flex-grow-1"></span>
            <span class="flex-shrink-0" v-if="!f.enabled">{{ $t("filter_paused") }}</span>
            <span
              class="flex-shrink-0 d-flex align-items-center gap-1"
              :title="$t('filters_hidden', { count: f.hidden })"
              v-else>
              <v-icon name="eye-off" />{{ f.hidden }}
            </span>
          </small>
        </div>
      </div>
    </div>

    <!-- filter editor -->
    <div class="filters-editor d-flex flex-column flex-grow-1" style="min-width: 0">
      <div class="px-2 py-1 d-flex gap-1 align-items-center">
        <button class="c-button-pill d-lg-none" @click="selected = null" :title="$t('filters')">
          <v-icon name="chevron-left" />
        </button>
        <div class="flex-grow-1 px-1 text-truncate user-select-none fw-semibold">
          <template v-if="selected === 'new'">{{ $t("new_filter") }}</template>
          <template v-else-if="selected !== null">{{ $t("edit_filter") }}</template>
        </div>
        <button
          class="c-button-pill text-danger"
          :title="$t('delete')"
          @click="deleteFilter()"
          v-if="typeof selected === 'number'">
          <v-icon name="trash" />
        </button>
        <button class="c-button-pill" @click="$emit('close')" :title="$t('close')">
          <v-icon name="x" />
        </button>
      </div>

      <div class="overflow-auto border-top flex-grow-1">
        <!-- empty state -->
        <div class="filters-intro mx-auto px-4 py-5" v-if="selected === null">
          <div class="filters-intro-icon mb-3"><v-icon name="filter" /></div>
          <h2 class="mb-2" style="font-size: 1.25rem">{{ $t("filter_empty_title") }}</h2>
          <p class="opacity-75">{{ $t("filter_empty_text") }}</p>
          <p class="small opacity-75 mb-2">{{ $t("filter_examples") }}</p>
          <div class="d-flex flex-wrap gap-2">
            <button
              class="c-button filter-example"
              v-for="example in examples"
              @click="newFilter({ pattern: example.pattern, match_content: example.content })">
              <code>{{ regexLiteral({ pattern: example.pattern, case_sensitive: false }) }}</code>
            </button>
          </div>
        </div>

        <!-- form & live preview -->
        <form class="filters-form mx-auto px-4 py-3" v-else @submit.prevent="save()">
          <div class="filter-sentence d-flex flex-wrap align-items-center gap-2 mb-2">
            <span>{{ $t("filter_hide_when") }}</span>
            <div class="c-segmented" role="group">
              <button
                type="button"
                :aria-pressed="fieldOf(form) === 'title'"
                @click="setField('title')">
                {{ $t("filter_field_title") }}
              </button>
              <button
                type="button"
                :aria-pressed="fieldOf(form) === 'content'"
                @click="setField('content')">
                {{ $t("filter_field_content") }}
              </button>
              <button
                type="button"
                :aria-pressed="fieldOf(form) === 'page'"
                @click="setField('page')">
                {{ $t("filter_field_page") }}
              </button>
            </div>
            <span>{{ $t("filter_matches") }}</span>
          </div>

          <div
            class="filter-regex d-flex align-items-center"
            :class="{ 'is-invalid': !!preview.error }">
            <span class="filter-regex-slash">/</span>
            <input
              class="flex-grow-1"
              ref="pattern"
              v-model="form.pattern"
              spellcheck="false"
              autocomplete="off"
              autocapitalize="off"
              :aria-invalid="!!preview.error"
              aria-describedby="filter-pattern-hint" />
            <span class="filter-regex-slash">/{{ form.case_sensitive ? "" : "i" }}</span>
            <button
              type="button"
              class="c-button-pill filter-case"
              :aria-pressed="form.case_sensitive"
              :title="$t('filter_case_sensitive')"
              @click="form.case_sensitive = !form.case_sensitive">
              Aa
            </button>
          </div>
          <div id="filter-pattern-hint" class="small mt-1 mb-3">
            <span class="text-danger" v-if="preview.error">{{ preview.error }}</span>
            <span class="opacity-75" v-else>{{ $t("filter_syntax_hint") }}</span>
          </div>

          <div
            class="d-flex flex-wrap align-items-center gap-2"
            :class="form.match_page ? 'mb-1' : 'mb-3'">
            <label for="filter-scope">{{ $t("filter_in") }}</label>
            <select
              id="filter-scope"
              class="c-input flex-grow-1"
              style="min-width: 0"
              v-model="form.feed_id">
              <option :value="null">{{ $t("all_feeds") }}</option>
              <optgroup v-for="group in feedGroups" :label="group.title">
                <option v-for="feed in group.feeds" :value="feed.id">{{ feed.title }}</option>
              </optgroup>
            </select>
          </div>
          <div class="small mb-3" v-if="form.match_page">
            <span class="text-danger" v-if="pageNeedsFeed">{{ $t("filter_page_needs_feed") }}</span>
            <span class="opacity-75" v-else>{{ $t("filter_page_hint") }}</span>
          </div>

          <div class="d-flex flex-wrap align-items-center gap-2">
            <label class="d-flex align-items-center gap-2 cursor-pointer user-select-none me-auto">
              <input type="checkbox" v-model="form.enabled" />
              {{ $t("filter_enabled") }}
            </label>
            <button
              type="button"
              class="c-button"
              @click="revert()"
              v-if="dirty && selected !== 'new'">
              {{ $t("revert") }}
            </button>
            <button type="submit" class="c-button c-button-primary" :disabled="!canSave">
              {{ $t("save") }}
            </button>
          </div>
          <div class="text-danger small mt-2" v-if="saveError">{{ saveError }}</div>

          <hr />

          <div
            class="d-flex align-items-baseline gap-2 mb-2 user-select-none"
            v-if="form.pattern.trim() && !preview.error">
            <span class="fw-semibold">{{ $t("filter_matching", { count: preview.count }) }}</span>
            <span class="small opacity-75" v-if="preview.count > preview.list.length">
              {{ $t("filter_preview_limit", { count: preview.list.length }) }}
            </span>
            <span class="c-spinner ms-auto" v-if="preview.loading"></span>
          </div>
          <div
            class="d-flex align-items-center gap-2 small opacity-75 mb-2 user-select-none"
            v-if="preview.pages?.fetching && !preview.error">
            <span class="c-spinner"></span>
            {{ $t("filter_pages_fetching", { count: preview.pages.missing }) }}
          </div>
          <p
            class="opacity-75"
            v-if="
              form.pattern.trim() &&
              !preview.loading &&
              !preview.error &&
              !preview.count &&
              !preview.pages?.fetching
            ">
            {{ $t("filter_no_matches") }}
          </p>
          <div class="d-flex flex-column gap-1" :class="{ 'opacity-50': preview.loading }">
            <div
              class="filter-match px-2 py-1"
              :class="{ 'is-starred': m.status === 'starred' }"
              :title="m.status === 'starred' ? $t('filter_starred_visible') : undefined"
              v-for="m in preview.list"
              :key="m.id">
              <div class="d-flex align-items-center small opacity-75 gap-1">
                <v-icon name="star-full" v-if="m.status === 'starred'" />
                <span class="flex-fill text-truncate">{{ feedsById[m.feed_id]?.title }}</span>
                <v-relative-time class="flex-shrink-0" :val="m.date" :locale="language" />
              </div>
              <div class="text-break" v-if="m.field === 'title'">
                {{ m.before }}<mark>{{ m.match }}</mark
                >{{ m.after }}
              </div>
              <template v-else>
                <div class="text-break">{{ m.title || $t("untitled") }}</div>
                <div class="text-break small opacity-75">
                  {{ m.before }}<mark>{{ m.match }}</mark
                  >{{ m.after }}
                </div>
              </template>
            </div>
          </div>
        </form>
      </div>
    </div>
  </div>
</template>

<script lang="ts">
import { defineComponent } from "vue";
import type { PropType } from "vue";
import api, { HTTPError } from "../api";
import { debounce, to } from "../utils";
import icon from "./icon.vue";
import relativeTime from "./relative-time.vue";
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

export default defineComponent({
  components: { "v-icon": icon, "v-relative-time": relativeTime },
  props: {
    filters: { type: Array as PropType<Filter[]>, required: true },
    feeds: { type: Array as PropType<Feed[]>, required: true },
    folders: { type: Array as PropType<Folder[]>, required: true },
    feedsById: { type: Object as PropType<Record<number, Feed>>, required: true },
    listWidth: { type: Number, required: true },
    language: { type: String, required: true },
    // a new filter to start editing, e.g. from "filter similar articles"
    draft: { type: Object as PropType<Partial<FilterParams> | null>, default: null },
  },
  emits: ["close", "changed"],
  data() {
    return {
      selected: null as number | "new" | null,
      form: emptyForm(),
      saving: false,
      saveError: "",
      preview: {
        loading: false,
        error: "",
        count: 0,
        list: [] as FilterPreviewMatch[],
        pages: undefined as FilterPreviewResponse["pages"],
      },
      previewSeq: 0,
      pagesPoll: undefined as ReturnType<typeof setTimeout> | undefined,
      examples: [
        { pattern: "sponsored|advertisement", content: false },
        { pattern: "^\\[(ad|promo)\\]", content: false },
        { pattern: "\\bgiveaway\\b", content: true },
      ],
    };
  },
  created() {
    this.schedulePreview = debounce(this.runPreview, PREVIEW_DELAY);
    if (this.draft) this.newFilter(this.draft);
  },
  beforeUnmount() {
    clearTimeout(this.pagesPoll);
  },
  computed: {
    original(): FilterParams | null {
      if (this.selected === "new") return emptyForm();
      const f = this.filters.find(f => f.id === this.selected);
      if (!f) return null;
      const { pattern, match_content, match_page, case_sensitive, feed_id, enabled } = f;
      return { pattern, match_content, match_page, case_sensitive, feed_id, enabled };
    },
    dirty(): boolean {
      return !!this.original && !sameForm(this.form, this.original);
    },
    canSave(): boolean {
      return (
        !this.saving &&
        !!this.form.pattern.trim() &&
        !this.preview.error &&
        !this.pageNeedsFeed &&
        (this.selected === "new" || this.dirty)
      );
    },
    // pages are fetched only for the feeds that need them
    pageNeedsFeed(): boolean {
      return this.form.match_page && this.form.feed_id === null;
    },
    totalHidden(): number {
      return this.filters.reduce((sum, f) => sum + (f.enabled ? f.hidden : 0), 0);
    },
    feedGroups(): { title: string; feeds: Feed[] }[] {
      const byFolder = new Map<number | null, Feed[]>();
      for (const feed of this.feeds) {
        if (!byFolder.has(feed.folder_id)) byFolder.set(feed.folder_id, []);
        byFolder.get(feed.folder_id)!.push(feed);
      }
      const groups = this.folders
        .filter(folder => byFolder.has(folder.id))
        .map(folder => ({ title: folder.title, feeds: byFolder.get(folder.id)! }));
      if (byFolder.has(null)) groups.push({ title: "──", feeds: byFolder.get(null)! });
      return groups;
    },
  },
  watch: {
    draft(draft: Partial<FilterParams> | null) {
      if (draft) this.newFilter(draft);
    },
    form: {
      deep: true,
      handler() {
        this.saveError = "";
        clearTimeout(this.pagesPoll);
        if (!this.form.pattern.trim() || this.pageNeedsFeed) {
          this.previewSeq++;
          this.preview = { loading: false, error: "", count: 0, list: [], pages: undefined };
          return;
        }
        this.preview.loading = true;
        this.schedulePreview();
      },
    },
  },
  methods: {
    schedulePreview() {},
    async runPreview() {
      if (!this.form.pattern.trim() || this.pageNeedsFeed) return;
      const seq = ++this.previewSeq;
      const [err, data] = await to(api.filters.preview({ ...this.form, enabled: true }));
      if (seq !== this.previewSeq) return; // the form changed in the meantime
      if (err) {
        this.preview = { loading: false, error: "", count: 0, list: [], pages: undefined };
        return;
      }
      const wasFetching = !!this.preview.pages?.fetching;
      this.preview = {
        loading: false,
        error: data.error || "",
        count: data.count,
        list: data.list,
        pages: data.pages,
      };
      if (data.pages?.fetching) {
        this.pagesPoll = setTimeout(() => this.runPreview(), PAGES_POLL_DELAY);
      } else if (wasFetching) {
        // the fetched pages may have changed what saved filters hide
        this.$emit("changed");
      }
    },
    fieldOf(f: Pick<FilterParams, "match_content" | "match_page">): Field {
      if (f.match_page) return "page";
      return f.match_content ? "content" : "title";
    },
    fieldLabel(f: Pick<FilterParams, "match_content" | "match_page">): string {
      const field = this.fieldOf(f);
      if (field === "page") return this.$t("filter_field_page");
      return field === "content" ? this.$t("filter_field_content") : this.$t("filter_field_title");
    },
    setField(field: Field) {
      this.form.match_content = field === "content";
      this.form.match_page = field === "page";
    },
    regexLiteral(f: Pick<FilterParams, "pattern" | "case_sensitive">): string {
      return "/" + f.pattern + "/" + (f.case_sensitive ? "" : "i");
    },
    scopeTitle(feedId: number | null): string {
      if (feedId === null) return this.$t("all_feeds");
      return this.feedsById[feedId]?.title || "";
    },
    focusPattern() {
      this.$nextTick(() => (this.$refs.pattern as HTMLInputElement | undefined)?.focus());
    },
    newFilter(draft: Partial<FilterParams> = {}) {
      this.selected = "new";
      this.form = { ...emptyForm(), ...draft };
      this.focusPattern();
    },
    selectFilter(f: Filter) {
      this.selected = f.id;
      this.revert();
    },
    revert() {
      if (this.original) this.form = { ...this.original };
    },
    async save() {
      if (!this.canSave) return;
      this.saving = true;
      const data = { ...this.form, pattern: this.form.pattern.trim() };
      const [err, saved] = await to(
        this.selected === "new"
          ? api.filters.create(data)
          : api.filters.update(this.selected as number, data),
      );
      this.saving = false;
      if (err) {
        this.saveError =
          this.$t("fail_save_filter") +
          (err instanceof HTTPError ? ` (${err.status} ${err.statusText})` : "");
        return;
      }
      this.form.pattern = data.pattern;
      this.$emit("changed");
      this.selected = saved.id;
    },
    async deleteFilter() {
      const f = this.filters.find(f => f.id === this.selected);
      if (!f || !confirm(this.$t("confirm_delete", { name: this.regexLiteral(f) }))) return;
      const [err] = await to(api.filters.delete(f.id));
      if (err) {
        this.saveError = this.$t("fail_save_filter");
        return;
      }
      this.selected = null;
      this.$emit("changed");
    },
  },
});
</script>
