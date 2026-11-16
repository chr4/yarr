import { useI18n } from "../i18n";
import type { Lang } from "../i18n";
import Dropdown from "./Dropdown";
import Icon from "./Icon";
import Tooltip from "./Tooltip";
import type { AppSettings, Theme } from "../models/settings";

const THEMES: Theme[] = ["light", "sepia", "night", "system"];

const UI_SIZES = [0.8, 0.9, 1, 1.1, 1.2, 1.3, 1.4, 1.5, 1.6];

const LANGUAGES: { code: Lang; name: string }[] = [
  { code: "en", name: "English" },
  { code: "de", name: "Deutsch" },
  { code: "es", name: "Español" },
  { code: "fr", name: "Français" },
  { code: "ja", name: "日本語" },
  { code: "pt", name: "Português" },
  { code: "ru", name: "Русский" },
  { code: "zh", name: "简体中文" },
];

interface DropdownAppProps {
  settings: AppSettings;
  onShowShortcuts: () => void;
  requiresAuth: boolean;
  onLogout: () => void;
  onUISizeChange: (size: number) => void;
}

export default function DropdownApp({
  settings,
  onShowShortcuts,
  requiresAuth,
  onLogout,
  onUISizeChange,
}: DropdownAppProps) {
  const { t } = useI18n();
  const sidebarCollapsed = settings.values.feed_list_collapsed.value;
  const uiSize = settings.values.ui_size.value;

  return (
    <Dropdown toggleClass="" drop="start" title={t("settings_main")} button={<Icon name="anchor" />}>
      <div className="c-dropdown-header" role="heading" aria-level={2}>
        {t("theme")}
      </div>
      <div className="row text-center m-0">
        {THEMES.map(th => (
          <Tooltip key={th} label={th} className="col-3 px-0">
            <button
              type="button"
              className={`c-button-link theme-swatch w-100 px-0 rounded-0 theme-${th}`}
              aria-label={th}
              aria-pressed={settings.values.theme_name.value === th}
              onClick={e => {
                e.stopPropagation();
                settings.update({ theme_name: th });
              }}
            />
          </Tooltip>
        ))}
      </div>

      <div className="c-dropdown-divider" />
      <div className="c-dropdown-header" role="heading" aria-level={2}>
        {t("ui_size")}
      </div>
      {/* a select rather than buttons: the menu grows and shrinks with the size, which moved
          buttons clicked repeatedly away from the pointer, onto other entries */}
      <div className="c-dropdown-item c-dropdown-select d-flex align-items-center gap-1">
        <span className="flex-grow-1">{Math.round(uiSize * 100)}%</span>
        <Icon className="flex-shrink-0" name="chevron-down" />
        <select
          aria-label={t("ui_size")}
          value={String(uiSize)}
          onClick={e => e.stopPropagation()}
          onChange={e => onUISizeChange(Number((e.target as HTMLSelectElement).value))}>
          {UI_SIZES.map(size => (
            <option key={size} value={String(size)}>
              {Math.round(size * 100)}%
            </option>
          ))}
        </select>
      </div>

      <div className="c-dropdown-divider" />
      <button
        type="button"
        className="c-dropdown-item w-100 text-start d-flex gap-1"
        onClick={onShowShortcuts}>
        <Icon className="me-1" name="help-circle" />
        {t("shortcuts")}
      </button>
      <button
        type="button"
        className="c-dropdown-item w-100 text-start d-none d-lg-flex gap-1"
        onClick={() => settings.update({ feed_list_collapsed: !sidebarCollapsed })}>
        <Icon className="me-1" name={sidebarCollapsed ? "sidebar-open" : "sidebar-close"} />
        {sidebarCollapsed ? t("expand_sidebar") : t("collapse_sidebar")}
      </button>

      <div className="c-dropdown-divider" />
      <div className="c-dropdown-header" role="heading" aria-level={2}>
        A / あ / 文
      </div>
      <div className="container">
        <div className="row">
          {LANGUAGES.map(lang => (
            <Tooltip key={lang.code} label={lang.name} className="col-3 px-0">
              <button
                type="button"
                className="c-dropdown-item text-center w-100"
                aria-label={lang.name}
                aria-pressed={settings.values.language.value === lang.code}
                onClick={e => {
                  e.stopPropagation();
                  settings.update({ language: lang.code });
                }}>
                {lang.code}
              </button>
            </Tooltip>
          ))}
        </div>
      </div>

      {requiresAuth && (
        <>
          <div className="c-dropdown-divider" />
          <button type="button" className="c-dropdown-item w-100 text-start" onClick={onLogout}>
            <Icon className="me-1" name="log-out" />
            {t("log_out")}
          </button>
        </>
      )}
    </Dropdown>
  );
}
