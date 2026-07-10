import { ref } from "vue";
import request from "@/utils/request";
import { DEFAULT_UPLOAD_POLICY, type UploadPolicy } from "@/utils/upload";

let cachedPolicy: UploadPolicy | null = null;
let pendingPolicy: Promise<UploadPolicy> | null = null;

export function useUploadPolicy() {
  const policy = ref<UploadPolicy>(cachedPolicy || DEFAULT_UPLOAD_POLICY);

  const loadPolicy = async () => {
    if (cachedPolicy) {
      policy.value = cachedPolicy;
      return cachedPolicy;
    }
    if (!pendingPolicy) {
      pendingPolicy = request
        .get<UploadPolicy>("/upload-policy")
        .then((res) => {
          cachedPolicy = res.data;
          return cachedPolicy;
        })
        .catch(() => DEFAULT_UPLOAD_POLICY)
        .finally(() => {
          pendingPolicy = null;
        });
    }
    policy.value = await pendingPolicy;
    return policy.value;
  };

  return { policy, loadPolicy };
}
