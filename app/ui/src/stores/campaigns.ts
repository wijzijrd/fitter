import { defineStore } from "pinia";
import { ref } from "vue";
import { campaignClient } from "../lib/connect";
import type { Campaign, JoinCampaignResponse } from "../gen/fitter/v1/campaign_pb";
import type { ActivityMetric } from "../gen/fitter/v1/common_pb";
import type { GoalFormInput } from "../lib/goalInput";
import { toCreateGoalInit } from "../lib/goalInput";
import { fromDate } from "../lib/timestamp";

export interface CreateCampaignInput {
  name: string;
  description: string;
  // Both set or both unset — a pooled target is optional (Task 7's server
  // rule); there is no "mode" any more.
  metric?: ActivityMetric;
  targetValue?: number;
  startDate: Date;
  endDate: Date;
}

export const useCampaignsStore = defineStore("campaigns", () => {
  const campaigns = ref<Campaign[]>([]);
  const loading = ref(false);

  async function fetchCampaigns(): Promise<void> {
    loading.value = true;
    try {
      const resp = await campaignClient.listCampaigns({});
      campaigns.value = resp.campaigns;
    } finally {
      loading.value = false;
    }
  }

  async function createCampaign(input: CreateCampaignInput): Promise<Campaign> {
    const resp = await campaignClient.createCampaign({
      name: input.name,
      description: input.description,
      metric: input.metric,
      targetValue: input.targetValue,
      startDate: fromDate(input.startDate),
      endDate: fromDate(input.endDate),
    });
    if (resp.campaign) campaigns.value.unshift(resp.campaign);
    return resp.campaign!;
  }

  // joinCampaign submits 1+ goals (from the join-campaign dialog's
  // ClauseBuilder-backed sub-forms) alongside membership — see
  // JoinCampaignRequest.goals in campaign_pb.ts.
  async function joinCampaign(campaignId: string, goals: GoalFormInput[]): Promise<JoinCampaignResponse> {
    const resp = await campaignClient.joinCampaign({
      campaignId,
      goals: goals.map((g) => toCreateGoalInit(g, campaignId)),
    });
    upsertLocal(resp.campaign);
    return resp;
  }

  async function leaveCampaign(campaignId: string): Promise<void> {
    await campaignClient.leaveCampaign({ campaignId });
    const c = campaigns.value.find((c) => c.id === campaignId);
    if (c) c.joined = false;
  }

  async function getCampaign(campaignId: string): Promise<Campaign> {
    const resp = await campaignClient.getCampaign({ campaignId });
    upsertLocal(resp.campaign);
    return resp.campaign!;
  }

  function upsertLocal(campaign: Campaign | undefined): void {
    if (!campaign) return;
    const idx = campaigns.value.findIndex((c) => c.id === campaign.id);
    if (idx === -1) campaigns.value.push(campaign);
    else campaigns.value[idx] = campaign;
  }

  return { campaigns, loading, fetchCampaigns, createCampaign, joinCampaign, leaveCampaign, getCampaign };
});
