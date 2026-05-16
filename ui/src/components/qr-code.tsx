import { useEffect, useMemo, useRef, useState } from "react";
import clsx from "clsx";
import QRCodeStyling from "qr-code-styling";

import { grow } from "@/utils/classes";

const QR_SIZE = 256;

export default function QrCode({ qrCode }: { qrCode: string }) {
  const ref = useRef(null);

  const [isQrMounted, setisQrMounted] = useState(false);

  const qrStyle = useMemo(() => new QRCodeStyling({
      cornersSquareOptions: {
        type: "extra-rounded",
      },
      dotsOptions: {
        type: "rounded",
      },
      height: QR_SIZE,
      imageOptions: {
        hideBackgroundDots: true,
        imageSize: 0.4,
        margin: 2,
      },
      margin: 10,
      qrOptions: {
        errorCorrectionLevel: "M",
      },
      type: "svg",
      width: QR_SIZE,
    }), []);

  useEffect(() => {
    if (ref.current && qrStyle) {
      qrStyle.append(ref.current);
      setisQrMounted(true);
    }
  }, [qrStyle]);

  useEffect(() => {
    if (!qrCode) {
      return;
    }
    if (qrStyle) {
      qrStyle.update({ data: qrCode });
    }
  }, [qrCode, qrStyle]);

  return (
    <div
      ref={ref}
      data-mounted={isQrMounted}
      className={clsx(
        grow,
        "size-full [&>svg>rect:nth-child(3)]:fill-foreground [&>svg>rect:nth-child(2)]:fill-surface",
      )}
    />
  );
}
