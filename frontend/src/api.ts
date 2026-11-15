// Thin re-exports of the generated Wails bindings, so pages import from one place.
export * as Api from "../bindings/github.com/1rowvy/drstats/app";
export type { ImportResult, AppInfo } from "../bindings/github.com/1rowvy/drstats/models";
export type { MatchRow, MatchMeta } from "../bindings/github.com/1rowvy/drstats/internal/store/models";
export type { MatchView, PlayerView, Farm, Metric, Line } from "../bindings/github.com/1rowvy/drstats/internal/stats/models";
export type { Death } from "../bindings/github.com/1rowvy/drstats/internal/parser/models";
export type { Settings } from "../bindings/github.com/1rowvy/drstats/internal/settings/models";
export type { Names } from "../bindings/github.com/1rowvy/drstats/internal/constants/models";
export type { Progress, ReplayEvent } from "../bindings/github.com/1rowvy/drstats/internal/library/models";
export * as AppEvents from "../bindings/events";
