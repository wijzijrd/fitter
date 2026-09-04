import { defineStore } from "pinia";
import { ref } from "vue";
import { campaignClient } from "../lib/connect";
import type { Campaign } from "../gen/fitter/v1/campaign_pb";
import { ActivityMetric, CampaignMode } from "../gen/fitter/v1/common_pb";
import { fromDate } from "../lib/timestamp";

export interface CreateCampaignInput {
  name: string;
  description: string;
  mode: CampaignMode;
  metric: ActivityMetric;
  targetValue: number;
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
      mode: input.mode,
      metric: input.metric,
      targetValue: input.targetValue,
      startDate: fromDate(input.startDate),
      endDate: fromDate(input.endDate),
    });
    if (resp.campaign) campaigns.value.unshift(resp.campaign);
    return resp.campaign!;
  }

  async function joinCampaign(campaignId: string, perMemberTargetValue?: number): Promise<void> {
    const resp = await campaignClient.joinCampaign({ campaignId, perMemberTargetValue });
    upsertLocal(resp.campaign);
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
