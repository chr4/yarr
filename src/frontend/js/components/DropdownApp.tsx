import { useI18n } from "../i18n";
import type { Lang } from "../i18n";
import Dropdown from "./Dropdown";
import Icon from "./Icon";
import Tooltip from "./Tooltip";
import type { Theme } from "./DropdownSettings";

const THEMES: Theme[] = ["light", "sepia", "night", "system"];

export const UI_SIZE_MIN = 0.8;
export const UI_SIZE_MAX = 1.6;

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
  theme: Theme;
  onThemeChange: (t: Theme) => void;
  onShowShortcuts: () => void;
  language: Lang;
  onLanguageChange: (lang: Lang) => void;
  requiresAuth: boolean;
  onLogout: () => void;
  sidebarCollapsed: boolean;
  onToggleSidebar: () => void;
  uiSize: number;
  onUISizeChange: (delta: number) => void;
}

export default function DropdownApp({
  theme,
  onThemeChange,
  onShowShortcuts,
  language,
  onLanguageChange,
  requiresAuth,
  onLogout,
  sidebarCollapsed,
  onToggleSidebar,
  uiSize,
  onUISizeChange,
}: DropdownAppProps) {
  const { t } = useI18n();

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
              aria-pressed={theme === th}
              onClick={e => {
                e.stopPropagation();
                onThemeChange(th);
              }}
            />
          </Tooltip>
        ))}
      </div>

      <div className="c-dropdown-divider" />
      <div className="c-dropdown-header" role="heading" aria-level={2}>
        {t("ui_size")}
      </div>
      <div className="row text-center m-0">
        <button
          type="button"
          className="c-dropdown-item col-4 px-0"
          style={{ fontSize: "0.8rem" }}
          disabled={uiSize <= UI_SIZE_MIN}
          onClick={e => {
            e.stopPropagation();
            onUISizeChange(-1);
          }}>
          A
        </button>
        <div className="col-4 d-flex align-items-center justify-content-center user-select-none">
          {Math.round(uiSize * 100)}%
        </div>
        <button
          type="button"
          className="c-dropdown-item col-4 px-0"
          style={{ fontSize: "1.2rem" }}
          disabled={uiSize >= UI_SIZE_MAX}
          onClick={e => {
            e.stopPropagation();
            onUISizeChange(1);
          }}>
          A
        </button>
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
        onClick={onToggleSidebar}>
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
                aria-pressed={language === lang.code}
                onClick={e => {
                  e.stopPropagation();
                  onLanguageChange(lang.code);
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
