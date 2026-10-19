import { defineStore } from "pinia";
import { reactive } from "vue";
import { feedClient } from "../lib/connect";
import type { FeedActivity, LeaderboardEntry, TimelineSeries } from "../gen/fitter/v1/feed_pb";

interface CampaignFeedState {
  activities: FeedActivity[];
  leaderboard: LeaderboardEntry[];
  pooledProgressValue: number;
  timelineSeries: TimelineSeries[];
  timelineTargetValue: number;
  loading: boolean;
}

function emptyState(): CampaignFeedState {
  return { activities: [], leaderboard: [], pooledProgressValue: 0, timelineSeries: [], timelineTargetValue: 0, loading: false };
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
    try {
      const [feed, leaderboard, timeline] = await Promise.all([
        feedClient.getCampaignFeed({ campaignId }),
        feedClient.getCampaignLeaderboard({ campaignId }),
        feedClient.getCampaignTimeline({ campaignId }),
      ]);
      s.activities = feed.activities;
      s.leaderboard = leaderboard.entries;
      s.pooledProgressValue = leaderboard.pooledProgressValue;
      s.timelineSeries = timeline.series;
      s.timelineTargetValue = timeline.targetValue;
    } finally {
      s.loading = false;
    }
  }

  // fetchLeaderboard is the light fetch the campaigns list uses for each bib;
  // it also warms the detail page's stack.
  async function fetchLeaderboard(campaignId: string): Promise<void> {
    const s = stateFor(campaignId);
    const resp = await feedClient.getCampaignLeaderboard({ campaignId });
    s.leaderboard = resp.entries;
    s.pooledProgressValue = resp.pooledProgressValue;
  }

  return { byCampaign, stateFor, fetchAll, fetchLeaderboard };
});
