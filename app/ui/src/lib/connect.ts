import { createClient, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { UserService } from "../gen/fitter/v1/user_pb";
import { CampaignService } from "../gen/fitter/v1/campaign_pb";
import { GoalService } from "../gen/fitter/v1/goal_pb";
import { FeedService } from "../gen/fitter/v1/feed_pb";
import { CatalogService } from "../gen/fitter/v1/catalog_pb";
import { getToken } from "./token";

const baseUrl = import.meta.env.VITE_API_BASE_URL as string;

const authInterceptor: Interceptor = (next) => async (req) => {
  const token = getToken();
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  return next(req);
};

const transport = createConnectTransport({
  baseUrl,
  interceptors: [authInterceptor],
});

export const userClient = createClient(UserService, transport);
export const campaignClient = createClient(CampaignService, transport);
export const goalClient = createClient(GoalService, transport);
export const feedClient = createClient(FeedService, transport);
export const catalogClient = createClient(CatalogService, transport);
