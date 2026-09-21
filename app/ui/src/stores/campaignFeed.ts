import { defineStore } from "pinia";
import { reactive } from "vue";
import { feedClient } from "../lib/connect";
import type {
  FeedActivity,
  LeaderboardGroup,
  TimelineGroup,
  TimelineSeries,
} from "../gen/fitter/v1/feed_pb";
import type { ActivityMetric } from "../gen/fitter/v1/common_pb";

interface CampaignFeedState {
  activities: FeedActivity[];
  leaderboardGroups: LeaderboardGroup[];
  pooledProgressValue?: number;
  pooledTargetValue?: number;
  pooledMetric?: ActivityMetric;
  timelineGroups: TimelineGroup[];
  pooledTimeline?: TimelineSeries;
  loading: boolean;
  loaded: boolean;
  error: string | null;
}

function emptyState(): CampaignFeedState {
  return {
    activities: [],
    leaderboardGroups: [],
    timelineGroups: [],
    loading: false,
    loaded: false,
    error: null,
  };
}

export const useCampaignFeedStore = defineStore("campaignFeed", () => {
  const byCampaign = reactive(new Map<string, CampaignFeedState>());

  function stateFor(campaignId: string): CampaignFeedState {
    let s = byCampaign.get(campaignId);
    if (!s) {
      s = emptyState();
      byCampaign.set(campaignId, s);
    }
    return s;
  }

  async function fetchAll(campaignId: string): Promise<void> {
    const s = stateFor(campaignId);
    s.loading = true;
    s.error = null;
    try {
      const [feed, leaderboard, timeline] = await Promise.all([
        feedClient.getCampaignFeed({ campaignId }),
        feedClient.getCampaignLeaderboard({ campaignId }),
        feedClient.getCampaignTimeline({ campaignId }),
      ]);
      s.activities = feed.activities;
      s.leaderboardGroups = leaderboard.groups;
      s.pooledProgressValue = leaderboard.pooledProgressValue;
      s.pooledTargetValue = leaderboard.pooledTargetValue;
      s.pooledMetric = leaderboard.pooledMetric;
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

  return { byCampaign, stateFor, fetchAll };
});
