#!/bin/bash
# Download WhatsApp APK for ReDroid installation
# Uses APKMirror or similar source

echo "Download the WhatsApp APK manually from:"
echo "  https://www.apkmirror.com/apk/whatsapp-inc/whatsapp/"
echo ""
echo "Choose the x86_64 variant for ReDroid compatibility."
echo "Save it to: /root/emulador/whatsapp/whatsapp.apk"
echo ""
echo "Then copy it to the Docker volume:"
echo "  docker run --rm -v whatsapp-apk:/apk -v /root/emulador/whatsapp:/src alpine cp /src/whatsapp.apk /apk/"
