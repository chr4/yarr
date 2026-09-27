<template>
  <div
    class="d-flex"
    :class="{
      'feed-selected': feedSelected !== null,
      'item-selected': itemSelected !== null,
    }">
    <!-- feed list -->
    <div
      id="col-feed-list"
      class="vh-100 position-relative d-flex flex-column border-end flex-shrink-0"
      :class="{ 'is-collapsed': feedListCollapsed }"
      :style="{ width: feedListWidth + 'px' }">
      <v-drag @resize="resizeFeedList" @reset="resetFeedList"></v-drag>
      <div class="feed-list-header px-2 py-1 d-flex align-items-center">
        <button
          class="c-button-pill feed-list-toggle"
          :aria-expanded="!feedListCollapsed"
          :title="feedListCollapsed ? $t('expand_sidebar') : $t('collapse_sidebar')"
          @click="feedListCollapsed = !feedListCollapsed">
          <v-icon name="anchor" />
        </button>
        <div class="flex-grow-1"></div>
        <button
          class="c-button-pill ms-1"
          :aria-pressed="filterSelected == 'unread'"
          :title="$t('unread')"
          @click="filterSelected = 'unread'">
          <v-icon name="circle-full" />
        </button>
        <button
          class="c-button-pill mx-1"
          :aria-pressed="filterSelected == 'starred'"
          :title="$t('starred')"
          @click="filterSelected = 'starred'">
          <v-icon name="star-full" />
        </button>
        <button
          class="c-button-pill me-1"
          :aria-pressed="filterSelected == ''"
          :title="$t('all')"
          @click="filterSelected = ''">
          <v-icon name="assorted" />
        </button>
        <div class="flex-grow-1"></div>
        <v-dropdown
          toggle-class="c-button-link c-button-pill px-2"
          ref="menuDropdown"
          drop="right"
          :title="$t('settings')">
          <template v-slot:button>
            <v-icon name="more-horizontal" />
          </template>

          <button class="c-dropdown-item w-100 text-start d-flex gap-1" @click="showModal = 'newfeed'">
            <v-icon class="me-1" name="plus" />
            {{ $t("new_feed") }}
          </button>
          <div class="c-dropdown-divider" v-if="refreshAvailable"></div>
          <button
            class="c-dropdown-item w-100 text-start d-flex gap-1"
            @click="fetchAllFeeds()"
            v-if="refreshAvailable">
            <v-icon class="me-1" name="rotate-cw" />
            {{ $t("refresh_feeds") }}
          </button>

          <div class="c-dropdown-divider"></div>

          <header class="c-dropdown-header" role="heading" aria-level="2">
            {{ $t("theme") }}
          </header>
          <div class="row text-center m-0">
            <button
              class="c-button-link theme-swatch col-3 px-0 rounded-0"
              :class="'theme-' + t"
              :title="t"
              :aria-label="t"
              :aria-pressed="theme.name == t"
              @click.stop="theme.name = t"
              v-for="t in ['light', 'sepia', 'night', 'system']"></button>
          </div>

          <div class="c-dropdown-divider"></div>

          <header class="c-dropdown-header" role="heading" aria-level="2">
            {{ $t("ui_size") }}
          </header>
          <div class="row text-center m-0">
            <button
              class="c-dropdown-item col-4 px-0"
              style="font-size: 0.8rem"
              :disabled="uiSize <= UI_SIZE_MIN"
              @click.stop="incrUISize(-1)">
              A
            </button>
            <div class="col-4 d-flex align-items-center justify-content-center user-select-none">
              {{ Math.round(uiSize * 100) }}%
            </div>
            <button
              class="c-dropdown-item col-4 px-0"
              style="font-size: 1.2rem"
              :disabled="uiSize >= UI_SIZE_MAX"
              @click.stop="incrUISize(1)">
              A
            </button>
          </div>

          <div class="c-dropdown-divider" v-if="refreshAvailable"></div>

          <header class="c-dropdown-header" role="heading" aria-level="2" v-if="refreshAvailable">
            {{ $t("auto_refresh") }}
          </header>
          <div class="row text-center m-0" v-if="refreshAvailable">
            <button
              class="c-dropdown-item col-4 px-0 d-flex gap-1 justify-content-center"
              @click.stop="changeRefreshRate(-1)"
              :disabled="!refreshRate">
              <v-icon name="chevron-down" />
            </button>
            <div class="col-4 d-flex align-items-center justify-content-center user-select-none">
              {{ refreshRateTitle }}
            </div>
            <button
              class="c-dropdown-item col-4 px-0 d-flex gap-1 justify-content-center"
              @click.stop="changeRefreshRate(1)"
              :disabled="refreshRate === refreshRateOptions[refreshRateOptions.length - 1].value">
              <v-icon name="chevron-up" />
            </button>
          </div>

          <div class="c-dropdown-divider"></div>

          <header class="c-dropdown-header" role="heading" aria-level="2">
            {{ $t("show_first") }}
          </header>
          <div class="d-flex">
            <button
              class="flex-fill c-dropdown-item text-center"
              :aria-pressed="itemSortNewestFirst"
              :class="{ active: itemSortNewestFirst }"
              @click.stop="itemSortNewestFirst = true">
              {{ $t("new") }}
            </button>
            <button
              class="flex-fill c-dropdown-item text-center"
              :aria-pressed="!itemSortNewestFirst"
              :class="{ active: !itemSortNewestFirst }"
              @click.stop="itemSortNewestFirst = false">
              {{ $t("old") }}
            </button>
          </div>
          <button
            class="c-dropdown-item w-100 text-start d-flex gap-1"
            :aria-pressed="markReadOnScroll"
            @click.stop="markReadOnScroll = !markReadOnScroll">
            <v-icon class="me-1" :name="markReadOnScroll ? 'check' : 'circle'" />
            {{ $t("mark_read_on_scroll") }}
          </button>
          <div class="c-dropdown-divider"></div>
          <header class="c-dropdown-header" role="heading" aria-level="2">
            {{ $t("subscriptions") }}
          </header>
          <form enctype="multipart/form-data" tabindex="-1" ref="opmlInputForm">
            <input
              type="file"
              id="opml-import"
              @change="importOPML"
              name="opml"
              style="opacity: 0; width: 1px; height: 0; position: absolute; z-index: -1" />
            <label
              class="c-dropdown-item mb-0 cursor-pointer w-100 d-flex gap-1"
              for="opml-import"
              @click.stop="">
              <v-icon class="me-1" name="download" />
              {{ $t("import") }}
            </label>
          </form>
          <a class="c-dropdown-item d-block text-start text-decoration-none d-flex gap-1" href="./opml/export">
            <v-icon class="me-1" name="upload" />
            {{ $t("export") }}
          </a>
          <div class="c-dropdown-divider"></div>
          <button class="c-dropdown-item w-100 text-start d-flex gap-1" @click="showModal = 'shortcuts'">
            <v-icon class="me-1" name="help-circle" />
            {{ $t("shortcuts") }}
          </button>
          <div class="c-dropdown-divider"></div>
          <header class="c-dropdown-header" role="heading" aria-level="2">A / あ / 文</header>
          <div class="container">
            <div class="row">
              <button
                v-for="lang in languages"
                class="c-dropdown-item text-center col-3 px-0"
                :aria-label="lang.name"
                :aria-pressed="language === lang.code"
                :title="lang.name"
                @click.stop="changeLanguage(lang.code)">
                {{ lang.code }}
              </button>
            </div>
          </div>
          <div class="c-dropdown-divider" v-if="requiresAuth"></div>
          <button class="c-dropdown-item w-100 text-start" v-if="requiresAuth" @click="logout()">
            <v-icon class="me-1" name="log-out" />
            {{ $t("log_out") }}
          </button>
        </v-dropdown>
      </div>
      <div id="feed-list-scroll" class="p-2 overflow-auto border-top flex-grow-1">
        <v-feedtree
          :tree="feedTree"
          v-model="feedSelected"
          :filter-selected="filterSelected"
          :stats="stats"
          :feed-errors="feed_errors"
          @toggle-folder="toggleFolderExpanded" />
      </div>
      <div
        class="feed-list-loading px-2 py-1 d-flex align-items-center border-top flex-shrink-0"
        v-if="loading.feeds">
        <span class="c-spinner mx-2"></span>
        <span class="text-truncate cursor-default user-select-none">{{
          $t("refreshing_progress", { count: loading.feeds })
        }}</span>
      </div>
    </div>
    <!-- item list -->
    <div
      id="col-item-list"
      class="vh-100 position-relative d-flex flex-column border-end flex-shrink-0"
      :style="{ width: itemListWidth + 'px' }">
      <v-drag @resize="resizeItemList" @reset="itemListWidth = 300"></v-drag>
      <div class="px-2 py-1 d-flex gap-1 align-items-center">
        <button
          class="c-button-pill d-md-none"
          @click="feedSelected = null"
          :title="$t('show_feeds')">
          <v-icon name="chevron-left" />
        </button>
        <div class="c-search flex-grow-1">
          <v-icon name="search" />
          <!-- id used by keybindings -->
          <input
            id="searchbar"
            class="d-block"
            v-model="itemSearch"
            :placeholder="$t('search_placeholder', { scope: searchScope })"
            @keydown.enter="($event.target as HTMLInputElement).blur()" />
        </div>
        <button
          class="c-button-pill"
          @click="markItemsRead()"
          v-if="filterSelected == 'unread'"
          :title="$t('mark_all_read')">
          <v-icon name="check" />
        </button>

        <button class="c-button-link c-button-pill px-2" v-if="!current.type" disabled>
          <v-icon name="more-horizontal" />
        </button>
        <v-dropdown
          toggle-class="c-button-link c-button-pill px-2"
          drop="right"
          :title="$t('feed_settings')"
          v-if="current?.feed?.id">
          <template v-slot:button>
            <v-icon name="more-horizontal" />
          </template>
          <header class="c-dropdown-header text-break line-clamp-3" role="heading" aria-level="2">
            {{ current?.feed?.title }}
          </header>
          <a
            class="c-dropdown-item text-start text-decoration-none d-flex gap-1"
            :href="current?.feed?.link"
            rel="noopener noreferrer"
            target="_blank"
            referrerpolicy="no-referrer"
            v-if="current?.feed?.link">
            <v-icon class="me-1" name="globe" />
            {{ $t("website") }}
          </a>
          <a
            class="c-dropdown-item text-start text-decoration-none d-flex gap-1"
            :href="current.feed.feed_link"
            rel="noopener noreferrer"
            target="_blank"
            referrerpolicy="no-referrer"
            v-if="current.feed.feed_link">
            <v-icon class="me-1" name="rss" />
            {{ $t("feed_link") }}
          </a>
          <div class="c-dropdown-divider" v-if="current.feed.link || current.feed.feed_link"></div>
          <button class="c-dropdown-item w-100 text-start d-flex gap-1" @click="renameFeed(current.feed)">
            <v-icon class="me-1" name="edit" />
            {{ $t("rename") }}
          </button>
          <button
            class="c-dropdown-item w-100 text-start d-flex gap-1"
            @click="updateFeedLink(current.feed)"
            v-if="current.feed.feed_link">
            <v-icon class="me-1" name="edit" />
            {{ $t("change_link") }}
          </button>
          <div class="c-dropdown-divider"></div>
          <header class="c-dropdown-header" role="heading" aria-level="2">
            {{ $t("move_to") }}
          </header>
          <template v-for="folder in folders">
            <button
              class="c-dropdown-item w-100 text-start d-flex gap-1"
              v-if="folder.id != current.feed.folder_id"
              @click="moveFeed(current.feed, folder.id)">
              <v-icon class="me-1" name="folder" />
              <span class="text-break line-clamp-3">{{ folder.title }}</span>
            </button>
          </template>
          <button
            class="c-dropdown-item w-100 text-start opacity-75 d-flex gap-1"
            @click="moveFeed(current.feed, null)"
            v-if="current.feed.folder_id">
            <v-icon class="me-1" name="folder-minus" />
            ──
          </button>
          <button
            class="c-dropdown-item w-100 text-start opacity-75 d-flex gap-1"
            @click="moveFeedToNewFolder(current.feed)">
            <v-icon class="me-1" name="folder-plus" />
            {{ $t("new_folder") }}
          </button>
          <div class="c-dropdown-divider"></div>
          <button
            class="c-dropdown-item w-100 text-start text-danger d-flex gap-1"
            @click.prevent="deleteFeed(current.feed)">
            <v-icon class="me-1" name="trash" />
            {{ $t("delete") }}
          </button>
        </v-dropdown>
        <v-dropdown
          toggle-class="c-button-link c-button-pill px-2"
          :title="$t('folder_settings')"
          drop="right"
          v-if="current?.folder?.id">
          <template v-slot:button>
            <v-icon name="more-horizontal" />
          </template>
          <header class="c-dropdown-header text-break line-clamp-3" role="heading" aria-level="2">
            {{ current?.folder?.title }}
          </header>
          <button class="c-dropdown-item w-100 text-start d-flex gap-1" @click="renameFolder(current.folder)">
            <v-icon class="me-1" name="edit" />
            {{ $t("rename") }}
          </button>
          <div class="c-dropdown-divider"></div>
          <button
            class="c-dropdown-item w-100 text-start text-danger d-flex gap-1"
            @click="deleteFolder(current.folder)">
            <v-icon class="me-1" name="trash" />
            {{ $t("delete") }}
          </button>
        </v-dropdown>
      </div>
      <div
        id="item-list-scroll"
        class="d-flex flex-column p-2 overflow-auto border-top flex-grow-1 gap-1"
        v-scroll="loadMoreItems"
        ref="itemlist">
        <a
          v-for="item in items"
          :key="item.id"
          :data-id="item.id"
          v-observe="readObserver"
          class="c-listitem d-flex flex-column user-select-none"
          :class="{ 'is-read': item.status === 'read' }"
          :href="item.link || undefined"
          target="_blank"
          rel="noopener noreferrer"
          draggable="false"
          role="radio"
          :aria-checked="itemSelected === item.id"
          @click="selectItem($event, item)">
          <div
            style="line-height: 100%; opacity: 0.7; margin-bottom: 0.1rem"
            class="d-flex align-items-center">
            <transition name="indicator">
              <v-icon
                :small="true"
                class="me-1"
                name="circle-full"
                v-if="item.status == 'unread'" />
              <v-icon
                :small="true"
                class="me-1"
                name="star-full"
                v-else-if="item.status == 'starred'" />
            </transition>
            <small class="flex-fill text-truncate me-1">
              {{ (feedsById[item.feed_id] || {}).title }}
            </small>
            <small class="flex-shrink-0"><v-relative-time :val="item.date" :locale="language" /></small>
          </div>
          <div class="text-break line-clamp-3">
            <span class="c-icon me-1" v-if="(feedsById[item.feed_id] || {}).icon">
              <img :src="feedsById[item.feed_id].icon || undefined" alt="" loading="lazy" />
            </span>
            <v-icon class="me-1" name="rss" v-else />{{ item.title || $t("untitled") }}
          </div>
        </a>
        <div class="text-center my-3" v-if="itemsHasMore">
          <span class="c-spinner"></span>
        </div>
        <!-- extra room, so that even the last article can be scrolled out of view -->
        <div
          class="flex-shrink-0"
          style="flex-basis: calc(100% + 1rem)"
          ref="itemlistSpacer"
          v-if="markReadOnScroll && items.length"></div>
      </div>
      <div
        class="px-3 py-2 border-top text-danger text-break"
        v-if="current?.feed?.id && feed_errors[current.feed.id]">
        {{ feed_errors[current.feed.id] }}
      </div>
    </div>
    <!-- item show -->
    <div id="col-item" class="vh-100 d-flex flex-column w-100" style="min-width: 0">
      <div class="px-2 py-1 d-flex gap-1 align-items-center" v-if="itemSelectedDetails">
        <button
          class="c-button-pill"
          @click="toggleItemStarred(itemSelectedDetails)"
          :title="$t('mark_starred')">
          <v-icon name="star-full" v-if="itemSelectedDetails.status == 'starred'" />
          <v-icon name="star" v-else />
        </button>
        <button
          class="c-button-pill"
          :title="$t('mark_unread')"
          @click="toggleItemRead(itemSelectedDetails)">
          <v-icon name="circle-full" v-if="itemSelectedDetails.status == 'unread'" />
          <v-icon name="circle" v-else />
        </button>
        <v-dropdown toggle-class="c-button-pill px-2" drop="center" :title="$t('appearance')">
          <template v-slot:button>
            <v-icon name="sliders" />
          </template>

          <button
            class="c-dropdown-item w-100 text-start font-sans-serif"
            :aria-pressed="theme.font == ''"
            @click.stop="theme.font = ''">
            {{ $t("sans_serif") }}
          </button>
          <button
            class="c-dropdown-item w-100 text-start font-serif"
            :aria-pressed="theme.font == 'serif'"
            @click.stop="theme.font = 'serif'">
            {{ $t("serif") }}
          </button>
          <button
            class="c-dropdown-item w-100 text-start font-monospace"
            :aria-pressed="theme.font == 'monospace'"
            @click.stop="theme.font = 'monospace'">
            {{ $t("monospace") }}
          </button>

          <div class="d-flex text-center">
            <button
              class="c-dropdown-item flex-fill"
              style="font-size: 0.8rem"
              @click.stop="incrFont(-1)">
              A
            </button>
            <button
              class="c-dropdown-item flex-fill"
              style="font-size: 1.2rem"
              @click.stop="incrFont(1)">
              A
            </button>
          </div>
        </v-dropdown>
        <button
          class="c-button-pill"
          :aria-pressed="!!itemSelectedReadability"
          @click="toggleReadability()"
          :title="$t('read_here')">
          <v-icon :class="{ 'is-loading': loading.readability }" name="book-open" />
        </button>
        <a
          class="c-button-pill"
          :href="itemSelectedDetails.link"
          rel="noopener noreferrer"
          target="_blank"
          referrerpolicy="no-referrer"
          :title="$t('open_link')">
          <v-icon name="external-link" />
        </a>
        <div class="flex-grow-1"></div>
        <button
          class="c-button-pill"
          @click="navigateToItem(-1)"
          :title="$t('previous_article')"
          :disabled="!items.length || itemSelected == items[0].id">
          <v-icon name="chevron-left" />
        </button>
        <button
          class="c-button-pill"
          @click="navigateToItem(+1)"
          :title="$t('next_article')"
          :disabled="!items.length || itemSelected == items[items.length - 1].id">
          <v-icon name="chevron-right" />
        </button>
        <button class="c-button-pill" @click="itemSelected = null" :title="$t('close_article')">
          <v-icon name="x" />
        </button>
      </div>
      <div
        v-if="itemSelectedDetails"
        ref="content"
        class="content px-4 pt-3 pb-5 border-top overflow-auto"
        :class="{
          'font-sans-serif': theme.font == '',
          'font-serif': theme.font == 'serif',
          'font-monospace': theme.font == 'monospace',
        }"
        :style="{ 'font-size': theme.size + 'rem' }">
        <div class="content-wrapper">
          <h1>
            <b>{{ itemSelectedDetails.title || $t("untitled") }}</b>
          </h1>
          <div class="opacity-50">
            <div>
              <span
                class="cursor-pointer"
                @click="feedSelected = 'feed:' + (feedsById[itemSelectedDetails.feed_id] || {}).id">
                {{ (feedsById[itemSelectedDetails.feed_id] || {}).title }}
              </span>
            </div>
            <time>{{ formatDate(itemSelectedDetails.date) }}</time>
          </div>
          <hr />
          <div v-if="!itemSelectedReadability">
            <div v-if="contentImages.length">
              <figure v-for="media in contentImages">
                <img :src="media.url" loading="lazy" />
                <figcaption v-if="media.description">
                  {{ media.description }}
                </figcaption>
              </figure>
            </div>
            <audio class="w-100" controls v-for="media in contentAudios" :src="media.url"></audio>
            <video class="w-100" controls v-for="media in contentVideos" :src="media.url"></video>
          </div>
          <div v-html="itemSelectedContent"></div>
        </div>
      </div>
    </div>
    <v-modal :open="showModal !== ''" @hide="showModal = ''">
      <button
        class="c-button-link outline-none position-absolute top-0 end-0 p-2 m-2"
        style="line-height: 1"
        @click="showModal = ''">
        <v-icon name="x" />
      </button>
      <v-newfeed
        v-if="showModal === 'newfeed'"
        :folders="folders"
        :folder-id="current?.feed?.folder_id ?? current?.folder?.id ?? null"
        @created="onFeedCreated"
        @folder-created="refreshFeeds" />
      <v-shortcuts v-else-if="showModal === 'shortcuts'" />
    </v-modal>
    <v-toast ref="toast" />
  </div>
</template>

<script lang="ts">
import type { Lang } from "../i18n";
import api, { NetworkError, HTTPError } from "../api";
import { scrollto, debounce, debounceMixin, to, dateTimeString } from "../utils";
import drag from "../components/drag.vue";
import dropdown from "../components/dropdown.vue";
import modal from "../components/modal.vue";
import shortcuts from "../components/shortcuts.vue";
import relativeTime from "../components/relative-time.vue";
import icon from "../components/icon.vue";
import feedTree from "../components/feedtree.vue";
import newfeed from "../components/newfeed.vue";
import toast from "../components/toast.vue";
import type { FeedTreeNode, TreeFeedNode, TreeFolderNode } from "../components/feedtree.vue";
import scrollDir from "../directives/scroll";
import focusDir from "../directives/focus";
import observeDir from "../directives/observe";
import { defineComponent, markRaw } from "vue";
import type {
  Feed,
  Folder,
  Item,
  FeedStat,
  MediaLink,
  ItemStatus,
  ItemListQuery,
  ItemMarkQuery,
} from "../api-types";

var app = window.app;

// room kept for the article view when resizing the feed and item lists
const MIN_ARTICLE_WIDTH = 300;

const UI_SIZE_MIN = 0.8;
const UI_SIZE_MAX = 1.6;

function clampColumnWidth(width: number, min: number, otherColumnWidth: number): number {
  const max = Math.max(min, window.innerWidth - otherColumnWidth - MIN_ARTICLE_WIDTH);
  return Math.min(Math.max(min, width), max);
}

const STATS_POLL_INTERVAL = 30 * 1000;

function sameFeedStats(a: Record<number, FeedStat>, b: Record<number, FeedStat>): boolean {
  const ids = Object.keys(b);
  if (Object.keys(a).length !== ids.length) return false;
  return ids.every(id => {
    const x = a[+id];
    const y = b[+id];
    return !!x && x.unread === y.unread && x.starred === y.starred;
  });
}

declare module "vue" {
  interface ComponentCustomProperties {
    $refs: {
      itemlist: HTMLElement;
      itemlistSpacer: HTMLElement | undefined;
      content: HTMLElement;
      opmlInputForm: HTMLFormElement;
      menuDropdown: InstanceType<typeof dropdown>;
      toast: InstanceType<typeof toast>;
    };
  }
}

type Theme = "system" | "light" | "sepia" | "night";
type ThemeFont = "" | "serif" | "monospace";
type Filter = "" | "starred" | "unread";
type SettingsLanguage = { code: Lang; name: string };
type Stats = { unread: number; starred: number };

var TITLE = document.title;

export default defineComponent({
  mixins: [debounceMixin],
  components: {
    "v-drag": drag,
    "v-dropdown": dropdown,
    "v-modal": modal,
    "v-shortcuts": shortcuts,
    "v-relative-time": relativeTime,
    "v-icon": icon,
    "v-feedtree": feedTree,
    "v-newfeed": newfeed,
    "v-toast": toast,
  },
  directives: {
    scroll: scrollDir,
    focus: focusDir,
    observe: observeDir,
  },
  async created() {
    this.updateMetaTheme();
    this.$t.set(document.documentElement.lang as Lang);

    // keep the theme-color meta tag in sync when the OS color scheme changes
    this._colorSchemeMql = window.matchMedia("(prefers-color-scheme: dark)");
    this._colorSchemeMql.addEventListener("change", this.updateMetaTheme);

    const [statsErr] = await to(this.refreshStats());
    if (statsErr) {
      this.$refs.toast.addToast(
        { title: this.$t("fail_load"), description: this.errDescription(statsErr) },
        { level: "fail", closeable: false },
      );
    }

    const [feedsErr] = await to(this.refreshFeeds());
    if (feedsErr) {
      this.$refs.toast.addToast(
        { title: this.$t("fail_load"), description: this.errDescription(feedsErr) },
        { level: "fail", closeable: false },
      );
    }

    if (!feedsErr) {
      this.refreshItems(false);
      this.computeStats();
    }

    // pick up feeds refreshed in the background (auto-refresh, other clients)
    this._pollTimer = window.setInterval(() => {
      if (document.visibilityState === "visible") this.refreshStats();
    }, STATS_POLL_INTERVAL);
    document.addEventListener("visibilitychange", this.onVisibilityChange);
  },
  mounted() {
    this.readObserver = new IntersectionObserver(this.onItemIntersect, {
      root: this.$refs.itemlist,
    });
  },
  beforeUnmount() {
    this.readObserver?.disconnect();
    this._colorSchemeMql?.removeEventListener("change", this.updateMetaTheme);
    document.removeEventListener("visibilitychange", this.onVisibilityChange);
    if (this._pollTimer) window.clearInterval(this._pollTimer);
    if (this._statsTimer) window.clearTimeout(this._statsTimer);
  },
  data() {
    var s = app.settings;
    return {
      filterSelected: s.filter as Filter,
      folders: [] as Folder[],
      feeds: [] as Feed[],
      feedSelected: s.feed,
      feedListWidth: s.feed_list_width || 300,
      feedListCollapsed: !!s.feed_list_collapsed,
      uiSize: (s.ui_size || 1) as number,
      UI_SIZE_MIN,
      UI_SIZE_MAX,
      items: [] as Item[],
      itemsHasMore: true,
      itemSelected: null as number | null,
      itemSelectedDetails: null as Item | null,
      itemSelectedReadability: "",
      itemSearch: "",
      itemSortNewestFirst: s.sort_newest_first as boolean,
      itemListWidth: s.item_list_width || 300,
      markReadOnScroll: !!s.mark_read_on_scroll,

      stats: { folders: {}, feeds: {}, total: { unread: 0, starred: 0 } } as {
        folders: Record<number, Stats>;
        feeds: Record<number, Stats>;
        total: Stats;
      },

      showModal: "" as "" | "shortcuts" | "newfeed",
      refreshAvailable: true,
      loading: {
        feeds: 0,
        items: false,
        readability: false,
      },
      fonts: ["", "serif", "monospace"] as ThemeFont[],
      feedStats: {} as Record<number, FeedStat>,
      theme: {
        name: s.theme_name as Theme,
        font: s.theme_font as ThemeFont,
        size: s.theme_size as number,
      },
      themeColors: {
        night: "#0e0e0e",
        sepia: "#f4f0e5",
        light: "#fff",
      },
      refreshRate: s.refresh_rate,
      requiresAuth: app.requiresAuth,
      feed_errors: {} as Record<number, string>,

      refreshRateOptions: [
        { title: "0", value: 0 },
        { title: "10m", value: 10 },
        { title: "30m", value: 30 },
        { title: "1h", value: 60 },
        { title: "2h", value: 120 },
        { title: "4h", value: 240 },
        { title: "12h", value: 720 },
        { title: "24h", value: 1440 },
      ],

      language: s.language,
      languages: [
        { code: "en", name: "English" },
        { code: "de", name: "Deutsch" },
        { code: "es", name: "Español" },
        { code: "fr", name: "Français" },
        { code: "ja", name: "日本語" },
        { code: "pt", name: "Português" },
        { code: "ru", name: "Русский" },
        { code: "zh", name: "简体中文" },
      ] as SettingsLanguage[],

      _colorSchemeMql: null as MediaQueryList | null,
      _statsTimer: null as number | null,
      _pollTimer: null as number | null,
      readObserver: null as IntersectionObserver | null,
      _seenItems: markRaw(new WeakSet<Element>()),
    };
  },
  computed: {
    feedTree(): FeedTreeNode[] {
      const [rootFeeds, folderFeeds] = this.feeds.reduce(
        (acc, f) => {
          acc[f.folder_id === null ? 0 : 1].push(f);
          return acc;
        },
        [[] as Feed[], [] as Feed[]],
      );

      const byFolder: Record<number, Feed[]> = folderFeeds.reduce(
        (acc, f) => {
          (acc[f.folder_id as number] ||= []).push(f);
          return acc;
        },
        {} as Record<number, Feed[]>,
      );

      const feedNode = (feed: Feed): TreeFeedNode => ({ type: "feed", feed });

      return [
        ...this.folders
          .filter(folder => !this.mustHideFolder(folder))
          .map(folder => {
            const feeds = (byFolder[folder.id] || [])
              .filter(f => !this.mustHideFeed(f))
              .map(feedNode);
            return { type: "folder" as const, folder, feeds };
          }),
        ...rootFeeds.filter(f => !this.mustHideFeed(f)).map(feedNode),
      ];
    },
    feedsById(): Record<number, Feed> {
      return this.feeds.reduce(
        (acc, f) => {
          acc[f.id] = f;
          return acc;
        },
        {} as Record<number, Feed>,
      );
    },
    foldersById(): Record<number, Folder> {
      return this.folders.reduce(
        (acc, f) => {
          acc[f.id] = f;
          return acc;
        },
        {} as Record<number, Folder>,
      );
    },
    current(): { type: string; feed: Feed | null; folder: Folder | null } {
      var parts = (this.feedSelected || "").split(":", 2);
      var type = parts[0];
      var guid = parts[1];

      const feed = type == "feed" ? this.feedsById[guid] : null;
      const folder = type == "folder" ? this.foldersById[guid] : null;

      return { type: type, feed: feed, folder: folder };
    },
    searchScope(): string {
      var type = (this.feedSelected || "").split(":", 2)[0];
      if (type == "feed")
        return (this.feedsById[this.feedSelected.split(":", 2)[1]] || {}).title || "";
      if (type == "folder")
        return (this.foldersById[this.feedSelected.split(":", 2)[1]] || {}).title || "";
      if (this.filterSelected == "unread") return this.$t("all_unread");
      if (this.filterSelected == "starred") return this.$t("all_starred");
      return this.$t("all_feeds");
    },
    itemSelectedContent(): string {
      if (!this.itemSelected) return "";
      if (this.itemSelectedReadability) return this.itemSelectedReadability;
      return this.itemSelectedDetails?.content || "";
    },
    contentImages(): MediaLink[] {
      if (!this.itemSelectedDetails) return [] as MediaLink[];
      return (this.itemSelectedDetails.media_links || []).filter(l => l.type === "image");
    },
    contentAudios(): MediaLink[] {
      if (!this.itemSelectedDetails) return [] as MediaLink[];
      return (this.itemSelectedDetails.media_links || []).filter(l => l.type === "audio");
    },
    contentVideos(): MediaLink[] {
      if (!this.itemSelectedDetails) return [] as MediaLink[];
      return (this.itemSelectedDetails.media_links || []).filter(l => l.type === "video");
    },
    refreshRateTitle() {
      const entry = this.refreshRateOptions.find(o => o.value === this.refreshRate);
      return entry ? entry.title : "0";
    },
  },
  watch: {
    theme: {
      deep: true,
      handler(theme) {
        this.updateMetaTheme();
        (async () => {
          const [err, _] = await to(
            api.settings.update({
              theme_name: theme.name,
              theme_font: theme.font,
              theme_size: theme.size,
            }),
          );
          if (err) {
            this.$refs.toast.addToast(
              { title: this.$t("fail_save_settings"), description: this.errDescription(err) },
              { level: "fail", closeable: false },
            );
          }
        })();
      },
    },
    feedStats: {
      deep: true,
      handler() {
        this.$debounce("watch:feedStats", this.computeStats, 500);
      },
    },
    async filterSelected(newVal, oldVal) {
      if (oldVal === undefined) return;
      this.itemSelected = null;
      this.items = [];
      this.itemsHasMore = true;

      const [err] = await to(api.settings.update({ filter: newVal }));
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_save_settings"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
        return;
      }
      this.refreshItems(false);
      this.computeStats();
    },
    async feedSelected(newVal, oldVal) {
      if (oldVal === undefined) return;
      this.itemSelected = null;
      this.items = [];
      this.itemsHasMore = true;

      this.refreshItems(false);
      if (this.$refs.itemlist) this.$refs.itemlist.scrollTop = 0;

      const [err] = await to(api.settings.update({ feed: newVal }));
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_save_settings"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
      }
    },
    async itemSelected(newVal, oldVal) {
      this.itemSelectedReadability = "";
      if (newVal === null) {
        this.itemSelectedDetails = null;
        return;
      }
      if (this.$refs.content) this.$refs.content.scrollTop = 0;

      const [itemErr, item] = await to(api.items.get(newVal));
      if (itemErr) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_load"), description: this.errDescription(itemErr) },
          { level: "fail", closeable: false },
        );
        return;
      }
      this.itemSelectedDetails = item;
      const details = this.itemSelectedDetails;
      if (details.status == "unread") {
        const [updateErr] = await to(api.items.update(details.id, { status: "read" }));
        if (updateErr) {
          this.$refs.toast.addToast(
            { title: this.$t("fail_update_article"), description: this.errDescription(updateErr) },
            { level: "fail", closeable: false },
          );
          return;
        }
        this.feedStats[details.feed_id].unread -= 1;
        var itemInList = this.items.find(i => i.id == item.id);
        if (itemInList) itemInList.status = "read";
        details.status = "read";
      }
    },
    itemSearch() {
      this.$debounce("watch:itemSearch", this.refreshItems, 500);
    },
    markReadOnScroll(newVal) {
      api.settings.update({ mark_read_on_scroll: newVal });
    },
    items() {
      // the old list is still rendered here. when everything was already scrolled past,
      // new items would end up out of sight above the viewport
      if (this.markReadOnScroll && this.itemListScrolledPast()) {
        this.$nextTick(this.scrollToFirstUnread);
      }
    },
    async itemSortNewestFirst(newVal, oldVal) {
      if (oldVal === undefined) return;
      const [err] = await to(api.settings.update({ sort_newest_first: newVal }));
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_save_settings"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
        return;
      }
      this.refreshItems(false);
    },
    feedListCollapsed(newVal) {
      api.settings.update({ feed_list_collapsed: newVal });
    },
    uiSize(newVal) {
      document.documentElement.style.setProperty("--v-ui-scale", String(newVal));
      api.settings.update({ ui_size: newVal });
    },
    feedListWidth: debounce(function (newVal, oldVal) {
      if (oldVal === undefined) return; // do nothing, initial setup
      api.settings.update({ feed_list_width: newVal });

      // const [err] = await to(api.settings.update({ feed_list_width: newVal }));
      // if (err) {
      //   this.$refs.toast.addToast(
      //     { title: this.$t("fail_save_settings"), description: this.errDescription(err) },
      //     { level: "fail", closeable: false },
      //   );
      // }
    }, 1000),
    itemListWidth: debounce(function (newVal, oldVal) {
      if (oldVal === undefined) return; // do nothing, initial setup
      api.settings.update({ item_list_width: newVal });

      // const [err] = await to(api.settings.update({ item_list_width: newVal }));
      // if (err) {
      //   this.$refs.toast.addToast(
      //     { title: this.$t("fail_save_settings"), description: this.errDescription(err) },
      //     { level: "fail", closeable: false },
      //   );
      // }
    }, 1000),
    refreshRate(newVal, oldVal) {
      if (oldVal === undefined) return;
      (async () => {
        const [err] = await to(api.settings.update({ refresh_rate: newVal }));
        if (err) {
          this.$refs.toast.addToast(
            { title: this.$t("fail_save_settings"), description: this.errDescription(err) },
            { level: "fail", closeable: false },
          );
        }
      })();
    },
  },
  methods: {
    updateMetaTheme() {
      let theme = this.theme.name;
      if (theme == "system") {
        var dark = window?.matchMedia("(prefers-color-scheme: dark)").matches;
        theme = dark ? "night" : "light";
      }
      const metaTag: HTMLMetaElement | null = document.querySelector("meta[name='theme-color']");
      metaTag && (metaTag.content = this.themeColors[theme]);

      document.documentElement.dataset.theme = this.theme.name;
    },
    onVisibilityChange() {
      if (document.visibilityState === "visible") this.refreshStats();
    },
    async refreshStats() {
      const [err, data] = await to(api.status());
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_load"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
        return;
      }

      this.loading.feeds = data.running;
      this.refreshAvailable = data.refresh;
      // poll while feeds are being refreshed, keeping a single polling loop
      if (data.running && !this._statsTimer) {
        this._statsTimer = window.setTimeout(() => {
          this._statsTimer = null;
          this.refreshStats();
        }, 500);
      }
      const oldStats = this.feedStats;
      const newStats: Record<number, FeedStat> = data.stats.reduce(
        (acc, stat) => ({ ...acc, [stat.feed_id]: stat }),
        {},
      );
      this.feedStats = newStats;
      if (Object.keys(oldStats).length && !sameFeedStats(oldStats, newStats)) {
        this.mergeNewItems();
      }

      const [feedErr, errors] = await to(api.feeds.list_errors());
      if (feedErr) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_load"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
        return;
      }
      this.feed_errors = errors;
    },
    getItemsQuery(): ItemListQuery {
      var query: ItemListQuery = {};
      if (this.feedSelected) {
        var parts = this.feedSelected.split(":", 2);
        var type = parts[0];
        var guid = parts[1];
        if (type == "feed") {
          query.feed_id = guid;
        } else if (type == "folder") {
          query.folder_id = guid;
        }
      }
      if (this.filterSelected) {
        query.status = this.filterSelected;
      }
      if (this.itemSearch) {
        query.search = this.itemSearch;
      }
      if (!this.itemSortNewestFirst) {
        query.oldest_first = true;
      }
      return query;
    },
    async refreshFeeds() {
      const [err, values] = await to(Promise.all([api.folders.list(), api.feeds.list()]));
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_load"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
        return;
      }
      this.folders = values[0];
      this.feeds = values[1];
    },
    async refreshItems(loadMore = false) {
      if (this.feedSelected === null) {
        this.items = [];
        this.itemsHasMore = false;
        return;
      }

      var query = this.getItemsQuery();
      if (loadMore) {
        query.after = this.items[this.items.length - 1].id;
      }

      this.loading.items = true;
      const [err, data] = await to(api.items.list(query));
      this.loading.items = false;
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_load"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
        return;
      }

      if (loadMore) {
        this.items = this.items.concat(data.list);
      } else {
        this.items = data.list;
      }
      this.itemsHasMore = data.has_more;

      // load more if there's some space left at the bottom of the item list.
      this.$nextTick(() => {
        if (this.itemsHasMore && !this.loading.items && this.itemListCloseToBottom()) {
          this.refreshItems(true);
        }
      });
    },
    async mergeNewItems() {
      // add items fetched in the background without resetting the list,
      // so that the scroll position, selection and loaded pages are kept
      if (this.feedSelected === null || this.loading.items) return;
      if (!this.items.length) return this.refreshItems(false);

      const query = this.getItemsQuery();
      const [err, data] = await to(api.items.list(query));
      if (err || this.loading.items) return;
      if (JSON.stringify(query) !== JSON.stringify(this.getItemsQuery())) return;

      const known = new Set(this.items.map(i => i.id));
      const fresh = data.list.filter(i => !known.has(i.id));
      // the whole first page is new, there may be a gap to the loaded items
      if (data.has_more && fresh.length === data.list.length) return this.refreshItems(false);

      // items sorted after the last loaded one arrive with the next page
      const last = this.items[this.items.length - 1];
      const added = this.itemsHasMore ? fresh.filter(i => this.compareItems(i, last) < 0) : fresh;
      if (added.length) {
        // keep the topmost visible entry in place when items are added above it
        const el = this.$refs.itemlist;
        const anchor = Array.from(el.children as HTMLCollectionOf<HTMLElement>).find(
          c => c.offsetTop + c.offsetHeight > el.scrollTop,
        );
        const anchorTop = anchor?.offsetTop ?? 0;
        this.items = this.items.concat(added).sort(this.compareItems);
        this.$nextTick(() => {
          if (anchor && el.scrollTop > 0) el.scrollTop += anchor.offsetTop - anchorTop;
        });
      }

      // new items beyond the first page of a fully loaded list
      if (!this.itemsHasMore && data.has_more) {
        this.itemsHasMore = true;
        this.refreshItems(true);
      }
    },
    compareItems(a: Item, b: Item): number {
      // same order as the server: by date, then by id
      const diff = Date.parse(a.date) - Date.parse(b.date) || a.id - b.id;
      return this.itemSortNewestFirst ? -diff : diff;
    },
    itemListCloseToBottom() {
      // approx. vertical space at the bottom of the list (loading el & paddings) when 1rem = 16px
      var bottomSpace = 70;
      var scale = (parseFloat(getComputedStyle(document.documentElement).fontSize) || 16) / 16;

      var el = this.$refs.itemlist;

      if (!el || el.scrollHeight === 0) return false; // element is invisible (responsive design)

      var spacerHeight = this.$refs.itemlistSpacer?.offsetHeight || 0;
      var closeToBottom =
        el.scrollHeight - el.scrollTop - el.offsetHeight - spacerHeight < bottomSpace * scale;
      return closeToBottom;
    },
    loadMoreItems() {
      if (!this.itemsHasMore) return;
      if (this.loading.items) return;
      if (this.itemListCloseToBottom()) return this.refreshItems(true);
      if (this.itemSelected && this.itemSelected === this.items[this.items.length - 1].id)
        return this.refreshItems(true);
    },
    async markItemsRead() {
      const markQuery = this.getItemsQuery();
      const query: ItemMarkQuery = {
        folder_id: markQuery.folder_id,
        feed_id: markQuery.feed_id,
      };
      const [err] = await to(api.items.mark_read(query));
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_update_article"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
        return;
      }
      this.items = [];
      this.itemSelected = null;
      this.itemsHasMore = false;
      this.refreshStats();
    },
    async toggleFolderExpanded(folder: Folder) {
      folder.is_expanded = !folder.is_expanded;
      const [err] = await to(api.folders.update(folder.id, { is_expanded: folder.is_expanded }));
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_save_folder"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
      }
    },
    formatDate(datestr: string) {
      return dateTimeString(new Date(datestr), this.language);
    },
    async moveFeed(feed: Feed, folder_id: number | null) {
      const [err] = await to(api.feeds.update(feed.id, { folder_id }));
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_save_feed"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
        return;
      }
      feed.folder_id = folder_id;
      this.refreshStats();
    },
    async moveFeedToNewFolder(feed: Feed) {
      const title = prompt(this.$t("prompt_folder_name"));
      if (!title) return;

      const [folderErr, folder] = await to(api.folders.create({ title: title }));
      if (folderErr) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_save_folder"), description: this.errDescription(folderErr) },
          { level: "fail", closeable: false },
        );
        return;
      }

      const [updateErr] = await to(api.feeds.update(feed.id, { folder_id: folder.id }));
      if (updateErr) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_save_feed"), description: this.errDescription(updateErr) },
          { level: "fail", closeable: false },
        );
        return;
      }

      const [feedsErr] = await to(this.refreshFeeds());
      if (!feedsErr) this.refreshStats();
    },
    async renameFolder(folder: Folder) {
      const newTitle = prompt(this.$t("prompt_new_title"), folder.title);
      if (newTitle) {
        const [err] = await to(api.folders.update(folder.id, { title: newTitle }));
        if (err) {
          this.$refs.toast.addToast(
            { title: this.$t("fail_save_folder"), description: this.errDescription(err) },
            { level: "fail", closeable: false },
          );
          return;
        }
        folder.title = newTitle;
        this.folders.sort((a, b) => a.title.localeCompare(b.title));
      }
    },
    async deleteFolder(folder: Folder) {
      if (confirm(this.$t("confirm_delete", { name: folder.title }))) {
        const [err] = await to(api.folders.delete(folder.id));
        if (err) {
          this.$refs.toast.addToast(
            { title: this.$t("fail_save_folder"), description: this.errDescription(err) },
            { level: "fail", closeable: false },
          );
          return;
        }
        this.feedSelected = null;
        this.refreshStats();
        this.refreshFeeds();
      }
    },
    async updateFeedLink(feed: Feed) {
      const newLink = prompt(this.$t("prompt_feed_link"), feed.feed_link);
      if (newLink !== null) {
        const [err] = await to(api.feeds.update(feed.id, { feed_link: newLink }));
        if (err) {
          this.$refs.toast.addToast(
            { title: this.$t("fail_save_feed"), description: this.errDescription(err) },
            { level: "fail", closeable: false },
          );
          return;
        }
        feed.feed_link = newLink;
      }
    },
    async renameFeed(feed: Feed) {
      const newTitle = prompt(this.$t("prompt_new_title"), feed.title);
      if (newTitle) {
        const [err] = await to(api.feeds.update(feed.id, { title: newTitle }));
        if (err) {
          this.$refs.toast.addToast(
            { title: this.$t("fail_save_feed"), description: this.errDescription(err) },
            { level: "fail", closeable: false },
          );
          return;
        }
        feed.title = newTitle;
      }
    },
    async deleteFeed(feed: Feed) {
      if (confirm(this.$t("confirm_delete", { name: feed.title }))) {
        const [err] = await to(api.feeds.delete(feed.id));
        if (err) {
          this.$refs.toast.addToast(
            { title: this.$t("fail_save_feed"), description: this.errDescription(err) },
            { level: "fail", closeable: false },
          );
          return;
        }
        this.feedSelected = null;
        this.refreshStats();
        this.refreshFeeds();
      }
    },
    onFeedCreated(feed: Feed) {
      this.refreshFeeds();
      this.refreshStats();
      this.showModal = "";
      this.feedSelected = "feed:" + feed.id;
    },
    selectItem(e: MouseEvent, item: Item) {
      // let modifier clicks (ctrl/cmd/shift) open the link natively
      if (item.link && (e.ctrlKey || e.metaKey || e.shiftKey)) return;
      e.preventDefault();
      this.itemSelected = item.id;
    },
    async toggleItemStatus(item: Item, targetstatus: ItemStatus) {
      const fallbackstatus: ItemStatus = "read";
      const oldstatus = item.status;
      const newstatus = item.status !== targetstatus ? targetstatus : fallbackstatus;

      const updateStats = (status: ItemStatus, incr: number) => {
        if (status == "unread" || status == "starred") {
          this.feedStats[item.feed_id][status] += incr;
        }
      };

      const [err] = await to(api.items.update(item.id, { status: newstatus }));
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_update_article"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
        return;
      }

      updateStats(oldstatus, -1);
      updateStats(newstatus, +1);

      var itemInList = this.items.find(i => i.id == item.id);
      if (itemInList) itemInList.status = newstatus;
      item.status = newstatus;
    },
    toggleItemStarred(item: Item) {
      this.toggleItemStatus(item, "starred");
    },
    toggleItemRead(item: Item) {
      this.toggleItemStatus(item, "unread");
    },
    itemListScrolledPast() {
      const el = this.$refs.itemlist;
      const items = el?.querySelectorAll(".c-listitem");
      if (!items?.length || !el.clientHeight) return false;
      const last = items[items.length - 1];
      return last.getBoundingClientRect().bottom <= el.getBoundingClientRect().top + el.clientTop;
    },
    scrollToFirstUnread() {
      const el = this.$refs.itemlist;
      const item = this.items.find(i => i.status === "unread");
      const target = item && el.querySelector(`[data-id="${item.id}"]`);
      if (!target) return;
      const offset = target.getBoundingClientRect().top - el.getBoundingClientRect().top;
      el.scrollTop += offset - el.clientTop - parseFloat(getComputedStyle(el).paddingTop);
    },
    onItemIntersect(entries: IntersectionObserverEntry[]) {
      const scrolledOut: Item[] = [];
      for (const entry of entries) {
        if (entry.isIntersecting) {
          this._seenItems.add(entry.target);
          continue;
        }
        // ignore items that were never on screen (e.g. new items inserted above the viewport)
        // and the hidden list on small screens
        if (!this._seenItems.has(entry.target) || !entry.rootBounds?.height) continue;
        // only items that left through the top were scrolled past
        if (entry.boundingClientRect.bottom > entry.rootBounds.top) continue;
        this._seenItems.delete(entry.target);
        const id = Number((entry.target as HTMLElement).dataset.id);
        const item = this.items.find(i => i.id === id);
        if (item?.status === "unread") scrolledOut.push(item);
      }
      if (this.markReadOnScroll && scrolledOut.length) this.markScrolledItemsRead(scrolledOut);
    },
    async markScrolledItemsRead(items: Item[]) {
      const setStatus = (item: Item, status: ItemStatus) => {
        item.status = status;
        if (this.itemSelectedDetails?.id === item.id) this.itemSelectedDetails.status = status;
        const stat = this.feedStats[item.feed_id];
        if (stat) stat.unread += status === "unread" ? 1 : -1;
      };

      // update optimistically, so that scrolling back and forth does not send duplicate requests
      items.forEach(item => setStatus(item, "read"));
      const results = await Promise.all(
        items.map(item => to(api.items.update(item.id, { status: "read" }))),
      );

      const failed = results.findIndex(([err]) => err);
      if (failed === -1) return;
      results.forEach(([err], i) => {
        if (err && items[i].status === "read") setStatus(items[i], "unread");
      });
      this.$refs.toast.addToast(
        {
          title: this.$t("fail_update_article"),
          description: this.errDescription(results[failed][0]),
        },
        { level: "fail", closeable: false },
      );
    },
    async importOPML(event: Event) {
      const input = event.target as HTMLInputElement;
      const form = this.$refs.opmlInputForm;
      this.$refs.menuDropdown.hide();
      const [err] = await to(api.upload_opml(form));
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_import"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
        return;
      }
      input.value = "";
      this.refreshFeeds();
      this.refreshStats();
    },
    async logout() {
      const [err] = await to(api.logout());
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_logout"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
        return;
      }
      document.location.reload();
    },
    async toggleReadability() {
      if (this.itemSelectedReadability) {
        this.itemSelectedReadability = "";
        return;
      }
      var item = this.itemSelectedDetails;
      if (!item?.link) return;
      this.loading.readability = true;
      const [err, data] = await to(api.crawl(item!.link));
      this.loading.readability = false;

      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_readability"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
      } else {
        this.itemSelectedReadability = data?.content || "";
      }
    },
    resizeFeedList(width: number) {
      // dragging the feed list narrow enough collapses it, dragging it back out expands it
      this.feedListCollapsed = width < 100;
      if (this.feedListCollapsed) return;
      this.feedListWidth = clampColumnWidth(width, 150, this.itemListWidth);
    },
    resetFeedList() {
      this.feedListCollapsed = false;
      this.feedListWidth = 300;
    },
    resizeItemList(width: number) {
      const feedListWidth = this.feedListCollapsed ? 0 : this.feedListWidth;
      this.itemListWidth = clampColumnWidth(width, 200, feedListWidth);
    },
    incrFont(x: number) {
      this.theme.size = +(this.theme.size + 0.1 * x).toFixed(1);
    },
    incrUISize(x: number) {
      const size = +(this.uiSize + 0.1 * x).toFixed(1);
      this.uiSize = Math.min(Math.max(UI_SIZE_MIN, size), UI_SIZE_MAX);
    },
    async fetchAllFeeds() {
      if (this.loading.feeds) return;
      const [err] = await to(api.feeds.refresh());
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_refresh"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
        return;
      }
      this.refreshStats();
    },
    computeStats() {
      let statsFeeds: Record<number, Stats> = {},
        statsFolders: Record<number, Stats> = {},
        statsTotal: Stats = { unread: 0, starred: 0 };

      for (var i = 0; i < this.feeds.length; i++) {
        const feed = this.feeds[i];
        if (!this.feedStats[feed.id]) continue;

        const n = this.feedStats[feed.id];
        const feedStats = { unread: n.unread || 0, starred: n.starred || 0 };

        statsFeeds[feed.id] = feedStats;

        if (feed.folder_id !== null) {
          if (!statsFolders[feed.folder_id])
            statsFolders[feed.folder_id] = { unread: 0, starred: 0 };
          statsFolders[feed.folder_id].unread += feedStats.unread;
          statsFolders[feed.folder_id].starred += feedStats.starred;
        }

        statsTotal.unread += feedStats.unread;
        statsTotal.starred += feedStats.starred;
      }

      this.stats = { feeds: statsFeeds, folders: statsFolders, total: statsTotal };

      const unread = this.stats.total.unread;
      document.title = TITLE + (unread ? ` (${unread})` : "");
    },
    // navigation helper, navigate relative to selected item
    navigateToItem(relativePosition: number) {
      let vm = this;
      if (this.itemSelected == null) {
        // if no item is selected, select first
        if (this.items.length !== 0) this.itemSelected = this.items[0].id;
        return;
      }

      var itemPosition = this.items.findIndex(x => {
        return x.id === this.itemSelected;
      });
      if (itemPosition === -1) {
        if (this.items.length !== 0) this.itemSelected = this.items[0].id;
        return;
      }

      var newPosition = itemPosition + relativePosition;
      if (newPosition < 0 || newPosition >= this.items.length) return;

      this.itemSelected = this.items[newPosition].id;

      this.$nextTick(() => {
        var scroll = document.querySelector("#item-list-scroll");
        var handle = scroll?.querySelector('[aria-checked="true"]');
        if (handle && scroll) scrollto(handle, scroll);

        this.loadMoreItems();
      });
    },
    // navigation helper, navigate relative to selected feed
    navigateToFeed(relativePosition: number) {
      const navigationList: string[] = [];
      for (const node of this.feedTree) {
        if (node.type === "folder") {
          navigationList.push("folder:" + node.folder.id);
          if (node.folder.is_expanded) {
            for (const feedNode of node.feeds) {
              navigationList.push("feed:" + feedNode.feed.id);
            }
          }
        } else {
          navigationList.push("feed:" + node.feed.id);
        }
      }
      navigationList.unshift("");

      var currentFeedPosition = navigationList.indexOf(this.feedSelected);

      if (currentFeedPosition == -1) {
        this.feedSelected = "";
        return;
      }

      var newPosition = currentFeedPosition + relativePosition;
      if (newPosition < 0 || newPosition >= navigationList.length) return;

      this.feedSelected = navigationList[newPosition];

      this.$nextTick(() => {
        var scroll = document.querySelector("#feed-list-scroll");
        var handle = scroll?.querySelector('[aria-checked="true"]');
        if (handle && scroll) scrollto(handle, scroll);
      });
    },
    changeRefreshRate(offset: number) {
      const curIdx = this.refreshRateOptions.findIndex(o => o.value === this.refreshRate);
      if (curIdx <= 0 && offset < 0) return;
      if (curIdx >= this.refreshRateOptions.length - 1 && offset > 0) return;
      this.refreshRate = this.refreshRateOptions[curIdx + offset].value;
    },
    mustHideFolder(folder: Folder): boolean {
      return !!(
        this.filterSelected &&
        !(this.current?.folder?.id === folder.id || this.current?.feed?.folder_id == folder.id) &&
        !this.stats.folders[folder.id]?.[this.filterSelected] &&
        (!this.itemSelectedDetails ||
          (this.feedsById[this.itemSelectedDetails.feed_id] || {}).folder_id != folder.id)
      );
    },
    mustHideFeed(feed: Feed): boolean {
      return !!(
        this.filterSelected &&
        !(this.current?.feed?.id === feed.id) &&
        !this.stats.feeds[feed.id]?.[this.filterSelected] &&
        (!this.itemSelectedDetails || this.itemSelectedDetails.feed_id != feed.id)
      );
    },
    async changeLanguage(lang: Lang) {
      this.$t.set(lang);
      this.language = lang;
      const [err] = await to(api.settings.update({ language: lang }));
      if (err) {
        this.$refs.toast.addToast(
          { title: this.$t("fail_save_settings"), description: this.errDescription(err) },
          { level: "fail", closeable: false },
        );
      }
    },
    errDescription(err: unknown): string | undefined {
      if (err instanceof HTTPError)
        return this.$t("error_server", { code: err.status, text: err.statusText });
      if (err instanceof NetworkError) return this.$t("error_network");
      return undefined;
    },
  },
});
</script>
