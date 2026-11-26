import { defineStore } from "pinia";
import { reactive } from "vue";
import { feedClient } from "../lib/connect";
import type {
  FeedActivity,
  LeaderboardGroup,
  PooledContribution,
  TimelineGroup,
  TimelineSeries,
  PersonStanding,
} from "../gen/fitter/v1/feed_pb";
import type { ActivityMetric } from "../gen/fitter/v1/common_pb";
import { LeaderboardWindow } from "../gen/fitter/v1/feed_pb";
import { toDate } from "../lib/timestamp";

interface CampaignFeedState {
  activities: FeedActivity[];
  // When the caller last looked at the feed, as of this load; unset on a first visit.
  seenAt?: Date;
  leaderboardGroups: LeaderboardGroup[];
  people: PersonStanding[];
  pooledProgressValue?: number;
  pooledTargetValue?: number;
  pooledMetric?: ActivityMetric;
  pooledContributions: PooledContribution[];
  timelineGroups: TimelineGroup[];
  pooledTimeline?: TimelineSeries;
  window: LeaderboardWindow;
  windowLoading: boolean;
  loading: boolean;
  loaded: boolean;
  error: string | null;
}

function emptyState(): CampaignFeedState {
  return {
    activities: [],
    leaderboardGroups: [],
    people: [],
    pooledContributions: [],
    timelineGroups: [],
    window: LeaderboardWindow.CAMPAIGN,
    windowLoading: false,
    loading: false,
    loaded: false,
    error: null,
  };
}

export const useCampaignFeedStore = defineStore("campaignFeed", () => {
  const byCampaign = reactive(new Map<string, CampaignFeedState>());

  function stateFor(campaignId: string): CampaignFeedState {
    // Read back through the reactive map so writes to a fresh entry still trigger updates.
    if (!byCampaign.has(campaignId)) byCampaign.set(campaignId, emptyState());
    return byCampaign.get(campaignId)!;
  }

  async function fetchAll(campaignId: string): Promise<void> {
    const s = stateFor(campaignId);
    s.loading = true;
    s.error = null;
    s.window = LeaderboardWindow.CAMPAIGN;
    try {
      const [feed, leaderboard, timeline] = await Promise.all([
        feedClient.getCampaignFeed({ campaignId }),
        feedClient.getCampaignLeaderboard({ campaignId }),
        feedClient.getCampaignTimeline({ campaignId }),
      ]);
      s.activities = feed.activities;
      s.seenAt = toDate(feed.seenAt) ?? undefined;
      s.leaderboardGroups = leaderboard.groups;
      s.people = leaderboard.people;
      s.pooledProgressValue = leaderboard.pooledProgressValue;
      s.pooledTargetValue = leaderboard.pooledTargetValue;
      s.pooledMetric = leaderboard.pooledMetric;
      s.pooledContributions = leaderboard.pooledContributions;
      s.timelineGroups = timeline.groups;
      s.pooledTimeline = timeline.pooled;
      s.loaded = true;
    } catch (e) {
      s.error = e instanceof Error ? e.message : "Failed to load campaign activity.";
      throw e;
    } finally {
      s.loading = false;
    }
  }

  // fetchLeaderboard is the light fetch the campaigns list uses for each bib;
  // it also warms the detail page's stack.
  async function fetchLeaderboard(campaignId: string, window = LeaderboardWindow.CAMPAIGN): Promise<void> {
    const s = stateFor(campaignId);
    const resp = await feedClient.getCampaignLeaderboard({ campaignId, window });
    s.window = window;
    s.leaderboardGroups = resp.groups;
    s.people = resp.people;
    s.pooledProgressValue = resp.pooledProgressValue;
    s.pooledTargetValue = resp.pooledTargetValue;
    s.pooledMetric = resp.pooledMetric;
    s.pooledContributions = resp.pooledContributions;
  }

  // setWindow re-scores the standings for a week, month or the whole campaign.
  async function setWindow(campaignId: string, window: LeaderboardWindow): Promise<void> {
    const s = stateFor(campaignId);
    s.windowLoading = true;
    try {
      await fetchLeaderboard(campaignId, window);
    } finally {
      s.windowLoading = false;
    }
  }

  // markSeen tells the server the caller has looked at the feed, then clears what was "new" so a
  // return to the tab reads "caught up". Best effort: a failure just leaves the divider as it was.
  async function markSeen(campaignId: string): Promise<void> {
    try {
      await feedClient.markFeedSeen({ campaignId });
      stateFor(campaignId).seenAt = new Date();
    } catch {
      /* the next visit will mark it */
    }
  }

  return { byCampaign, stateFor, fetchAll, fetchLeaderboard, setWindow, markSeen };
});
