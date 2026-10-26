"use client";

import { useCallback, useEffect, useState } from "react";
import { ArrowUpRight, CheckCircle2, KeyRound, RefreshCw } from "lucide-react";
import { api } from "@/lib/api";
import { useI18n } from "@/i18n/I18nProvider";
import { RechargeModal } from "@/components/RechargeModal";

type RechargeConfig = {
  payment_enabled: boolean;
  card_recharge_enabled: boolean;
  payment_compute_rate: number;
  recharge_shop_enabled: boolean;
  recharge_shop_url: string;
};

export default function RechargePage() {
  const { ts, formatNumber } = useI18n();
  const [config, setConfig] = useState<RechargeConfig | null>(null);
  const [code, setCode] = useState("");
  const [loading, setLoading] = useState(false);
  const [success, setSuccess] = useState("");
  const [error, setError] = useState("");
  const [configError, setConfigError] = useState(false);
  const [onlinePaymentOpen, setOnlinePaymentOpen] = useState(false);
  const shopURL = config?.recharge_shop_url?.trim() || "";

  const load = useCallback(async () => {
    setConfigError(false);
    try {
      setConfig(await api<RechargeConfig>("/api/payment/config"));
    } catch {
      setConfigError(true);
    }
  }, []);

  useEffect(() => { void load(); }, [load]);

  const redeem = async (event: React.FormEvent) => {
    event.preventDefault();
    if (!code.trim() || loading) return;
    setLoading(true);
    setSuccess("");
    setError("");
    try {
      const result = await api<{ credited: number }>("/api/recharge/card", {
        method: "POST",
        body: JSON.stringify({ code: code.trim() }),
      });
      setSuccess(ts(`兑换成功，${formatNumber(result.credited, { maximumFractionDigits: 6 })} 算力已到账。`));
      setCode("");
      await load();
      window.dispatchEvent(new Event("starai:wallet-changed"));
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : ts("兑换失败，请检查卡密后重试。"));
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="mcdl-page page-padding min-h-full py-6 sm:py-8">
      {configError ? (
        <div role="alert" className="mb-6 flex flex-wrap items-center justify-between gap-3 rounded-2xl bg-red-50 px-5 py-4 text-sm text-red-700 dark:bg-red-500/10 dark:text-red-200">
          <span>{ts("充值配置暂时加载失败，请重试。")}</span>
          <button type="button" onClick={() => void load()} className="inline-flex items-center gap-2 rounded-xl bg-white px-3 py-2 font-medium shadow-sm dark:bg-white/10"><RefreshCw size={14} />{ts("重新加载")}</button>
        </div>
      ) : null}

      <section className="soft-card min-w-0 overflow-hidden">
        <div className="p-5 sm:p-6">
          <form onSubmit={redeem}>
            <div className="flex flex-col gap-4 xl:flex-row xl:items-center xl:gap-6">
              <div className="flex items-center gap-3 xl:min-w-[17rem]">
                <span className="flex h-11 w-11 shrink-0 items-center justify-center rounded-2xl bg-primary/10 text-primary"><KeyRound size={21} /></span>
                <div>
                  <h2 className="font-semibold text-gray-950 dark:text-gray-100">{ts("卡密兑换")}</h2>
                  <p className="mt-0.5 text-xs text-gray-400">{ts("输入购买后获得的完整卡密")}</p>
                </div>
              </div>
              <div className="flex w-full flex-1 flex-col gap-3 sm:flex-row sm:items-center">
                <input
                  id="recharge-code"
                  type="text"
                  autoComplete="off"
                  spellCheck={false}
                  value={code}
                  onChange={(event) => setCode(event.target.value.toUpperCase())}
                  placeholder={ts("请输入卡密")}
                  className="w-full flex-1 rounded-xl border border-gray-200 bg-white px-4 py-3 font-mono text-sm uppercase text-gray-900 outline-none transition focus:border-primary focus:ring-2 focus:ring-primary/15 dark:border-white/10 dark:bg-white/5 dark:text-gray-100"
                />
                <button type="submit" disabled={loading || !code.trim() || config?.card_recharge_enabled !== true} className="w-full shrink-0 rounded-xl bg-primary px-10 py-3 text-sm font-semibold text-dark transition hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-50 sm:w-auto">
                  {loading ? ts("兑换中...") : ts("立即兑换")}
                </button>
              </div>
            </div>
            {error ? <p role="alert" className="mt-3 rounded-xl bg-red-50 px-3 py-2.5 text-sm text-red-600 dark:bg-red-500/10 dark:text-red-200">{error}</p> : null}
            {success ? <p role="status" className="mt-3 flex items-start gap-2 rounded-xl bg-emerald-50 px-3 py-2.5 text-sm text-emerald-700 dark:bg-emerald-500/10 dark:text-emerald-200"><CheckCircle2 size={16} className="mt-0.5 shrink-0" />{success}</p> : null}
          </form>
          {shopURL || config?.payment_enabled ? (
            <div className="mt-4 flex flex-wrap items-center gap-3 border-t border-gray-100 pt-4 dark:border-white/10">
              {shopURL ? (
                <a href={shopURL} target="_blank" rel="noopener noreferrer" className="inline-flex items-center gap-1.5 rounded-xl bg-gray-100 px-3 py-2 text-xs font-medium text-gray-600 hover:bg-gray-200 dark:bg-white/10 dark:text-gray-200 dark:hover:bg-white/15">
                  {ts("新窗口打开店铺")} <ArrowUpRight size={14} />
                </a>
              ) : null}
              {config?.payment_enabled ? (
                <button
                  type="button"
                  onClick={() => setOnlinePaymentOpen(true)}
                  className="ml-auto shrink-0 rounded-xl bg-gray-900 px-5 py-2.5 text-sm font-semibold text-white transition hover:bg-gray-800 dark:bg-white dark:text-gray-950 dark:hover:bg-gray-100"
                >
                  {ts("使用站内在线支付")}
                </button>
              ) : null}
            </div>
          ) : null}
        </div>
        {config?.recharge_shop_enabled === true && shopURL ? (
          <iframe
            src={shopURL}
            title={ts("卡密店铺")}
            className="block h-[calc(100vh-15rem)] min-h-[38rem] w-full border-x-0 border-b-0 border-t border-gray-100 bg-white dark:border-white/10"
            referrerPolicy="strict-origin-when-cross-origin"
          />
        ) : config ? (
          <div className="flex min-h-[28rem] items-center justify-center border-t border-gray-100 px-6 text-center text-sm text-gray-400 dark:border-white/10">{ts("卡密店铺暂未开放。")}</div>
        ) : (
          <div className="flex min-h-[28rem] items-center justify-center border-t border-gray-100 px-6 text-center text-sm text-gray-400 dark:border-white/10">{ts("正在加载卡密店铺...")}</div>
        )}
      </section>

      <RechargeModal
        open={onlinePaymentOpen}
        showCardRecharge={false}
        onClose={() => setOnlinePaymentOpen(false)}
        onSuccess={() => {
          void load();
          window.dispatchEvent(new Event("starai:wallet-changed"));
        }}
      />
    </div>
  );
}
