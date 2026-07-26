import { ref } from "vue";
import type { CBLUser, CBLBan } from "~/types/player";

interface CBLBanEdge {
  node: CBLBan | null;
}
interface CBLBanConnection {
  edges: CBLBanEdge[] | null;
}
type CBLUserRaw = Omit<CBLUser, "activeBans" | "expiredBans"> & {
  activeBans: CBLBanConnection | null;
  expiredBans: CBLBanConnection | null;
};

interface CBLGraphQLResponse {
  data: {
    steamUser: CBLUserRaw | null;
  };
  errors?: any[];
}

/**
 * Community Ban List reputation lookup.
 * Third-party GraphQL endpoint; intentionally uses native fetch (no auth
 * header must ever be sent there); failures resolve to null, never throw.
 */
export function useCBL() {
  const cblData = ref<CBLUser | null>(null);
  const cblLoading = ref(false);

  async function fetchCBLData(steamId: string) {
    cblLoading.value = true;

    const query = `
      query Search($id: String!) {
        steamUser(id: $id) {
          id
          name
          avatarFull
          reputationPoints
          riskRating
          reputationRank
          lastRefreshedInfo
          lastRefreshedReputationPoints
          lastRefreshedReputationRank
          reputationPointsMonthChange
          activeBans: bans(orderBy: "created", orderDirection: DESC, expired: false) {
            edges {
              node {
                id
                created
                expires
                reason
                banList { id name organisation { id name } }
              }
            }
          }
          expiredBans: bans(orderBy: "created", orderDirection: DESC, expired: true) {
            edges {
              node {
                id
                created
                expires
                reason
                banList { id name organisation { id name } }
              }
            }
          }
        }
      }
    `;

    try {
      const response = await fetch("https://communitybanlist.com/graphql", {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          query,
          variables: {
            id: steamId,
          },
        }),
      });

      if (!response.ok) {
        throw new Error("Failed to fetch CBL data");
      }

      const result: CBLGraphQLResponse = await response.json();

      if (result.errors && result.errors.length > 0) {
        throw new Error("GraphQL errors occurred");
      }

      const raw = result.data.steamUser;
      if (raw) {
        const flatten = (conn: CBLBanConnection | null): CBLBan[] =>
          (conn?.edges ?? [])
            .map((e) => e.node)
            .filter((n): n is CBLBan => n !== null);
        cblData.value = {
          ...raw,
          activeBans: flatten(raw.activeBans),
          expiredBans: flatten(raw.expiredBans),
        };
      } else {
        cblData.value = null;
      }
    } catch (err: any) {
      console.error("Failed to fetch CBL data:", err);
      cblData.value = null;
    } finally {
      cblLoading.value = false;
    }
  }

  function reset() {
    cblData.value = null;
  }

  return { cblData, cblLoading, fetchCBLData, reset };
}
