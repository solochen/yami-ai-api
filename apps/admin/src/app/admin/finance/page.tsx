"use client";

import { useEffect, useMemo, useState } from "react";
import {
  BadgePercent,
  Coins,
  ReceiptText,
  ShieldAlert,
  TrendingUp,
  WalletCards,
} from "lucide-react";
import { adminApi } from "@/lib/api";
import type { FinancialSummary } from "@starai/shared-types";

const credits = (v?: number) =>
  `${(v ?? 0).toLocaleString("zh-CN", { maximumFractionDigits: 2 })} 算力`;
const yuan = (v?: number) =>
  `¥${(v ?? 0).toLocaleString("zh-CN", { minimumFractionDigits: 2, maximumFractionDigits: 2 })}`;
const percent = (v?: number) =>
  `${((v ?? 0) * 100).toLocaleString("zh-CN", { maximumFractionDigits: 2 })}%`;

export default function FinancePage() {
  const [summary, setSummary] = useState<FinancialSummary | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    adminApi<FinancialSummary>("/finance")
      .then(setSummary)
      .catch((err) => setError(err?.message || "财务数据加载失败"));
  }, []);

  const margin = useMemo(() => summary?.realized_margin_rate ?? 0, [summary]);

  const cards = [
    { label: "用户消费（算力）", value: credits(summary?.user_charge_compute), icon: WalletCards },
    { label: "上游成本（算力）", value: credits(summary?.provider_cost_compute), icon: ReceiptText },
    { label: "已实现佣金（算力）", value: credits(summary?.commission_compute), icon: TrendingUp },
    { label: "实际毛利率", value: percent(margin), icon: BadgePercent },
    { label: "配置佣金率", value: `${(summary?.gross_margin_percent ?? 0).toFixed(2)}%`, icon: BadgePercent },
    { label: "结算笔数", value: String(summary?.settlement_count ?? 0), icon: Coins },
  ];

  const liabilities = [
    { label: "用户钱包余额（算力）", value: credits(summary?.wallet_balance_compute) },
    { label: "冻结占用（算力）", value: credits(summary?.frozen_compute) },
    { label: "未履行负债（算力）", value: credits(summary?.unfulfilled_liability) },
    { label: "已收现金（人民币）", value: yuan(summary?.recharge_cash_paid) },
    { label: "在线充值到账（算力）", value: credits(summary?.cash_recharge_credits) },
    { label: "卡密兑换到账（算力）", value: credits(summary?.card_recharge_credits) },
    { label: "已兑换卡密面值（人民币）", value: yuan(summary?.card_face_value_cny) },
  ];

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-2xl font-bold text-gray-950">平台财务</h1>
          <p className="mt-1 text-sm text-gray-500">
            对账口径：用户售价 − 上游成本 = 平台佣金。所有算力金额保留 6 位小数向上取整。
          </p>
        </div>
      </div>

      {error && (
        <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800">
          {error}
        </div>
      )}

      <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
        {cards.map((item) => {
          const Icon = item.icon;
          return (
            <div key={item.label} className="rounded-2xl border border-gray-200 bg-white p-5">
              <div className="flex items-center justify-between">
                <span className="text-sm text-gray-500">{item.label}</span>
                <Icon size={18} className="text-gray-400" />
              </div>
              <div className="mt-3 text-2xl font-bold text-gray-950">{item.value}</div>
            </div>
          );
        })}
      </div>

      <div className="rounded-2xl border border-gray-200 bg-white p-5">
        <h2 className="text-sm font-semibold text-gray-950">负债与充值对账</h2>
        <div className="mt-4 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-4">
          {liabilities.map((item) => (
            <div key={item.label} className="rounded-xl bg-gray-50 px-4 py-3">
              <div className="text-xs text-gray-500">{item.label}</div>
              <div className="mt-1 text-lg font-semibold text-gray-900">{item.value}</div>
            </div>
          ))}
        </div>
      </div>

      {(summary?.incomplete_cost_count ?? 0) > 0 || (summary?.missing_route_cost_models ?? 0) > 0 ? (
        <div className="flex items-start gap-3 rounded-2xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-800">
          <ShieldAlert size={18} className="mt-0.5 shrink-0" />
          <div>
            <div className="font-medium">成本完整性问题</div>
            <div className="mt-1">
              已结算记录中存在 {summary?.incomplete_cost_count ?? 0} 笔成本不完整；
              另有 {summary?.missing_route_cost_models ?? 0} 个启用模型缺少可计费上游线路。
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
