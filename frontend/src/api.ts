// Thin re-exports of the generated Wails bindings, so pages import from one place.
export * as Api from "../bindings/github.com/1rowvy/drstats/app";
export type { ImportResult, AppInfo } from "../bindings/github.com/1rowvy/drstats/models";
export type { MatchRow, MatchMeta, DeathPoint, ItemTiming as ItemAvgRow, AbilityTotal } from "../bindings/github.com/1rowvy/drstats/internal/store/models";
export type {
  MatchView, PlayerView, DeathCtx, Farm, Metric, Line, Laning, LaneSnap, Gold, IdlePeriod, ItemTiming, Fights, Amount,
  Abilities, AbilityUse, Vision, TimeShare, Dashboard, Filter, Bucket, Habit, TrendGame, MetricAvg, Game, HeroOption,
  Battle, FightHero, ObjectiveView, Grade, Extras, Record as RecordRow, DayBucket, FastItemView, Streak,
  LaneHero, LaneStats, LaneMatchup, LaneOpp, FlowStats, PhaseStats, SwingGame,
  PlanView, GoalProgress, PlanGoal, FarmStats, FarmPhase, IdleZone,
  RankView, RankPoint, RankMark, RankBucket, Reason, MatchReasons, DeathStats, SmokeView, SmokeStats,
} from "../bindings/github.com/1rowvy/drstats/internal/stats/models";
export type { Death, Ward, Rune } from "../bindings/github.com/1rowvy/drstats/internal/parser/models";
export type { Settings } from "../bindings/github.com/1rowvy/drstats/internal/settings/models";
export type { Names } from "../bindings/github.com/1rowvy/drstats/internal/constants/models";
export type { Progress, ReplayEvent, ParseProgress } from "../bindings/github.com/1rowvy/drstats/internal/library/models";
export type { Progress as DownloadProgress } from "../bindings/github.com/1rowvy/drstats/internal/download/models";
export type { State as AutoState } from "../bindings/github.com/1rowvy/drstats/internal/autodl/models";
export * as AppEvents from "../bindings/events";
