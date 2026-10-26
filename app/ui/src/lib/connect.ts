import { Code, ConnectError, createClient, type Interceptor } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { UserService } from "../gen/fitter/v1/user_pb";
import { CampaignService } from "../gen/fitter/v1/campaign_pb";
import { GoalService } from "../gen/fitter/v1/goal_pb";
import { FeedService } from "../gen/fitter/v1/feed_pb";
import { CatalogService } from "../gen/fitter/v1/catalog_pb";
import { apiBaseUrl } from "./config";
import { saveReturnTo } from "./returnTo";
import { clearToken, getToken } from "./token";

const authInterceptor: Interceptor = (next) => async (req) => {
  const token = getToken();
  if (token) {
    req.header.set("Authorization", `Bearer ${token}`);
  }
  try {
    return await next(req);
  } catch (err) {
    // An expired or revoked session can't recover in place; start a clean login.
    if (token && ConnectError.from(err).code === Code.Unauthenticated) {
      clearToken();
      saveReturnTo(window.location.pathname + window.location.search);
      window.location.assign("/login");
    }
    throw err;
  }
};

const transport = createConnectTransport({
  baseUrl: apiBaseUrl,
  interceptors: [authInterceptor],
});

export const userClient = createClient(UserService, transport);
export const campaignClient = createClient(CampaignService, transport);
export const goalClient = createClient(GoalService, transport);
export const feedClient = createClient(FeedService, transport);
export const catalogClient = createClient(CatalogService, transport);
