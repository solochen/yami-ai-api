import { redirect } from "next/navigation";

// 工具箱已并入智能体，保留该路径用于兼容旧链接与书签。
export default function ToolboxPage() {
  redirect("/app");
}
