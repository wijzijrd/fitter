import { Code, ConnectError } from "@connectrpc/connect";

// errorMessage turns an RPC failure into copy a person can act on. Server
// messages are developer-facing ("target_value must be positive"), so they're
// never shown as-is.
export function errorMessage(err: unknown, fallback: string): string {
  switch (ConnectError.from(err).code) {
    case Code.InvalidArgument:
      return "Some of those details aren't valid. Check them and try again.";
    case Code.NotFound:
      return "That doesn't exist any more. It may have been deleted.";
    case Code.PermissionDenied:
      return "You don't have access to that.";
    case Code.Unavailable:
      return "Can't reach Fitter right now. Check your connection and try again.";
    default:
      return fallback;
  }
}
