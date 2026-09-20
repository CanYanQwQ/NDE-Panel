import { useState } from "react";
import { Button } from "@heroui/button";
import { QRCodeSVG } from "qrcode.react";

/**
 * 订阅链接二维码。
 *
 * 手机端(小火箭 / v2rayNG / Clash)大多支持扫码直接添加订阅,
 * 比让车友在手机上手打一长串 token 靠谱得多。
 *
 * 注意:二维码底色写死白色、码点写死黑色 —— 跟着主题走的话深色模式下
 * 会变成"黑底白码",不少手机相机扫不出来(反色码不是所有解码器都认)。
 */
export function SubQr({ url, size = 200 }: { url: string; size?: number }) {
  if (!url) return null;
  return (
    <div className="flex flex-col items-center gap-2">
      <div className="bg-white p-3 rounded-lg shadow-sm">
        <QRCodeSVG value={url} size={size} level="M" bgColor="#ffffff" fgColor="#000000" />
      </div>
      <div className="text-xs text-default-400">手机客户端扫这个码直接添加订阅</div>
    </div>
  );
}

/**
 * 「📱 扫码」按钮 + 就地展开的二维码。
 * 用在一屏里有多条线路的地方(我的订阅 / 用户管理的订阅线路),
 * 不用弹窗——那两处本身就在弹窗里,嵌套弹窗容易出焦点问题。
 */
export function SubQrToggle({ url, size = 180 }: { url: string; size?: number }) {
  const [open, setOpen] = useState(false);
  if (!url) return null;
  return (
    <div className="flex flex-col gap-2 items-start">
      <Button size="sm" variant="flat" onPress={() => setOpen(!open)}>
        {open ? "收起二维码" : "📱 扫码"}
      </Button>
      {open && <SubQr url={url} size={size} />}
    </div>
  );
}
