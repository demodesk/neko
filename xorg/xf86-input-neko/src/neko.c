/*
 * (c) 2017 Martin Kepplinger <martink@posteo.de>
 * (c) 2007 Clement Chauplannaz, Thales e-Transactions <chauplac@gmail.com>
 * (c) 2006 Sascha Hauer, Pengutronix <s.hauer@pengutronix.de>
 *
 * derived from the xf86-input-void driver
 * Copyright 1999 by Frederic Lepied, France. <Lepied@XFree86.org>
 *
 * Permission to use, copy, modify, distribute, and sell this software and its
 * documentation for any purpose is  hereby granted without fee, provided that
 * the  above copyright   notice appear  in   all  copies and  that both  that
 * copyright  notice   and   this  permission   notice  appear  in  supporting
 * documentation, and that   the  name of  Frederic   Lepied not  be  used  in
 * advertising or publicity pertaining to distribution of the software without
 * specific,  written      prior  permission.     Frederic  Lepied   makes  no
 * representations about the suitability of this software for any purpose.  It
 * is provided "as is" without express or implied warranty.
 *
 * FREDERIC  LEPIED DISCLAIMS ALL   WARRANTIES WITH REGARD  TO  THIS SOFTWARE,
 * INCLUDING ALL IMPLIED   WARRANTIES OF MERCHANTABILITY  AND   FITNESS, IN NO
 * EVENT  SHALL FREDERIC  LEPIED BE   LIABLE   FOR ANY  SPECIAL, INDIRECT   OR
 * CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE,
 * DATA  OR PROFITS, WHETHER  IN  AN ACTION OF  CONTRACT,  NEGLIGENCE OR OTHER
 * TORTIOUS  ACTION, ARISING    OUT OF OR   IN  CONNECTION  WITH THE USE    OR
 * PERFORMANCE OF THIS SOFTWARE.
 *
 * SPDX-License-Identifier: MIT
 * License-Filename: COPYING
 */

/* neko input driver */

#ifdef HAVE_CONFIG_H
#include "config.h"
#endif

#define SOCKET_NAME "/tmp/resol.sock"
#define BUFFER_SIZE 12

#include <stdio.h>
#include <stdio.h>
#include <sys/socket.h>
#include <sys/un.h>
#include <misc.h>
#include <xf86.h>
#if !defined(DGUX)
#include <xisb.h>
#endif
#include <xf86_OSproc.h>
#include <xf86Xinput.h>
#include <exevents.h> /* Needed for InitValuator/Proximity stuff */
#include <X11/keysym.h>
#include <mipointer.h>
#include <xserver-properties.h>
#include <pthread.h>

#define TOUCH_MAX_SLOTS 10
#define MAXBUTTONS 11    /* > 10 */
#define TOUCH_NUM_AXES 3 /* x, y, pressure */

struct neko_sample {
    int type;
    int touchId;
    int x;
    int y;
    unsigned int pressure;
};

struct neko_priv {
    pthread_t thread;
    int height;
    int width;
    int pmax;
    ValuatorMask *valuators;
    int8_t abs_x_only;
    uint16_t slots;

    struct sockaddr_un addr;
    int listen_socket;
};

// from binary representation to struct
static void unpackNekoSample(struct neko_sample *samp, unsigned char *buffer)
{
    samp->type = buffer[0];
    samp->touchId = buffer[1];
    samp->x = buffer[2] << 8 | buffer[3];
    samp->y = buffer[4] << 8 | buffer[5];
    samp->pressure = buffer[6] << 8 | buffer[7];
}

static void xf86NekoReadInput(InputInfoPtr local)
{
    struct neko_priv *priv = (struct neko_priv *) (local->private);
    struct neko_sample samp;
    int ret;

    int data_socket;
    unsigned char buffer[BUFFER_SIZE];

    /* This is the main loop for handling connections. */

    for (;;) {

        /* Wait for incoming connection. */

        data_socket = accept(priv->listen_socket, NULL, NULL);
        if (data_socket == -1) {
            perror("accept");
            exit(EXIT_FAILURE);
        }

        fprintf(stderr, "xf86-input-neko: accepted\n");

        for(;;) {

            /* Wait for next data packet. */

            ret = read(data_socket, buffer, BUFFER_SIZE);
            if (ret == -1) {
                perror("read");
                exit(EXIT_FAILURE);
            }

            if (ret == 0) {
                fprintf(stderr, "xf86-input-neko: read 0 bytes\n");
                break;
            }

            fprintf(stderr, "xf86-input-neko: read %d bytes\n", ret);

            unpackNekoSample(&samp, buffer);

            ValuatorMask *m = priv->valuators;
            valuator_mask_zero(m);
            valuator_mask_set_double(m, 0, samp.x);
            valuator_mask_set_double(m, 1, samp.y);
            valuator_mask_set_double(m, 2, samp.pressure);

            xf86PostTouchEvent(local->dev, samp.touchId, samp.type, 0, m);
            fprintf(stderr, "xf86-input-neko: touchId is %d, type is %d\n", samp.touchId, samp.type);
            fprintf(stderr, "xf86-input-neko: x is %d, y is %d, pressure is %d\n", samp.x, samp.y, samp.pressure);
        }

        /* Close socket. */
        close(data_socket);
    }
}

static int xf86NekoControlProc(DeviceIntPtr device, int what)
{
    InputInfoPtr pInfo;
    unsigned char map[MAXBUTTONS + 1];
    Atom labels[MAXBUTTONS];
    Atom axis_labels[TOUCH_NUM_AXES];
    int i;
    struct neko_priv *priv;

    pInfo = device->public.devicePrivate;
    priv = pInfo->private;

    switch (what) {
    case DEVICE_INIT:
        device->public.on = FALSE;

        /* init button map */
        memset(map, 0, sizeof(map));
        for (i = 0; i < MAXBUTTONS; i++)
            map[i + 1] = i + 1;

        /* init labels */
        memset(labels, 0, ARRAY_SIZE(labels) * sizeof(Atom));
        labels[0] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_LEFT);
        labels[1] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_MIDDLE);
        labels[2] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_RIGHT);
        labels[3] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_WHEEL_UP);
        labels[4] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_WHEEL_DOWN);
        labels[5] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_HWHEEL_LEFT);
        labels[6] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_HWHEEL_RIGHT);
        labels[7] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_SIDE);
        labels[8] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_EXTRA);
        labels[9] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_FORWARD);
        labels[10] = XIGetKnownProperty(BTN_LABEL_PROP_BTN_BACK);

        /* init axis labels */
        memset(axis_labels, 0, ARRAY_SIZE(axis_labels) * sizeof(Atom));
        if (priv->abs_x_only) {
            axis_labels[0] = XIGetKnownProperty(AXIS_LABEL_PROP_ABS_X);
            axis_labels[1] = XIGetKnownProperty(AXIS_LABEL_PROP_ABS_Y);
            axis_labels[2] = XIGetKnownProperty(AXIS_LABEL_PROP_ABS_PRESSURE);
        } else {
            axis_labels[0] = XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_POSITION_X);
            axis_labels[1] = XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_POSITION_Y);
            axis_labels[2] = XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_PRESSURE);
        }

        if (InitButtonClassDeviceStruct(device,
                MAXBUTTONS,
                labels,
                map) == FALSE) {
            xf86IDrvMsg(pInfo, X_ERROR,
                "unable to allocate Button class device\n");
            return !Success;
        }

        if (InitValuatorClassDeviceStruct(device,
                TOUCH_NUM_AXES,
                axis_labels,
                0, Absolute) == FALSE) {
            xf86IDrvMsg(pInfo, X_ERROR,
                "unable to allocate Valuator class device\n");
            return !Success;
        }

        if (priv->abs_x_only) {
            InitValuatorAxisStruct(device, 0,
                XIGetKnownProperty(AXIS_LABEL_PROP_ABS_X),
                0,                /* min val */
                priv->width - 1,  /* max val */
                priv->width,      /* resolution */
                0,                /* min_res */
                priv->width,      /* max_res */
                Absolute);

            InitValuatorAxisStruct(device, 1,
                XIGetKnownProperty(AXIS_LABEL_PROP_ABS_Y),
                0,                /* min val */
                priv->height - 1, /* max val */
                priv->height,     /* resolution */
                0,                /* min_res */
                priv->height,     /* max_res */
                Absolute);

            InitValuatorAxisStruct(device, 2,
                XIGetKnownProperty(AXIS_LABEL_PROP_ABS_PRESSURE),
                0,                /* min val */
                priv->pmax,       /* max val */
                priv->pmax + 1,   /* resolution */
                0,                /* min_res */
                priv->pmax + 1,   /* max_res */
                Absolute);
        } else {
            InitValuatorAxisStruct(device, 0,
                XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_POSITION_X),
                0,                /* min val */
                priv->width - 1,  /* max val */
                priv->width,      /* resolution */
                0,                /* min_res */
                priv->width,      /* max_res */
                Absolute);

            InitValuatorAxisStruct(device, 1,
                XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_POSITION_Y),
                0,                /* min val */
                priv->height - 1, /* max val */
                priv->height,     /* resolution */
                0,                /* min_res */
                priv->height,     /* max_res */
                Absolute);

            InitValuatorAxisStruct(device, 2,
                XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_PRESSURE),
                0,                /* min val */
                priv->pmax,       /* max val */
                priv->pmax + 1,   /* resolution */
                0,                /* min_res */
                priv->pmax + 1,   /* max_res */
                Absolute);
        }

        if (InitTouchClassDeviceStruct(device,
                priv->slots,
                XIDirectTouch,
                TOUCH_NUM_AXES) == FALSE) {
            xf86IDrvMsg(pInfo, X_ERROR,
                "Unable to allocate TouchClassDeviceStruct\n");
            return !Success;
        }

        break;

    case DEVICE_ON:
        xf86IDrvMsg(pInfo, X_PROBED, "xf86-input-neko: DEVICE ON\n");
        device->public.on = TRUE;

        if (priv->thread == 0)
            pthread_create(&priv->thread, NULL, (void *)xf86NekoReadInput, pInfo);
        break;

    case DEVICE_OFF:
    case DEVICE_CLOSE:
        xf86IDrvMsg(pInfo, X_PROBED, "xf86-input-neko: DEVICE OFF\n");
        device->public.on = FALSE;
        break;
    }

    return Success;
}

static void xf86NekoUninit(__attribute__ ((unused)) InputDriverPtr drv,
            InputInfoPtr pInfo,
            __attribute__ ((unused)) int flags)
{
    struct neko_priv *priv = (struct neko_priv *)(pInfo->private);

    /* close socket */
    close(priv->listen_socket);
    unlink(SOCKET_NAME);

    if (priv->thread) {
        pthread_cancel(priv->thread);
        pthread_join(priv->thread, NULL);
        priv->thread = 0;
    }

    valuator_mask_free(&priv->valuators);
    xf86NekoControlProc(pInfo->dev, DEVICE_OFF);
    free(pInfo->private);
    pInfo->private = NULL;
    xf86DeleteInput(pInfo, 0);
}

static int xf86NekoInit(__attribute__ ((unused)) InputDriverPtr drv,
            InputInfoPtr pInfo,
            __attribute__ ((unused)) int flags)
{
    struct neko_priv *priv;
    char *s;

    priv = calloc(1, sizeof (struct neko_priv));
    if (!priv)
        return BadValue;

    pInfo->type_name = XI_TOUCHSCREEN;
    pInfo->control_proc = NULL;
    pInfo->read_input = NULL;
    pInfo->device_control = xf86NekoControlProc;
    pInfo->switch_mode = NULL;
    pInfo->private = priv;
    pInfo->dev = NULL;
    pInfo->fd = -1;

    s = xf86SetStrOption(pInfo->options, "path", NULL);
    if (!s)
        s = xf86SetStrOption(pInfo->options, "Device", NULL);

    {
        int ret;

        /*
        * In case the program exited inadvertently on the last run,
        * remove the socket.
        */

        unlink(SOCKET_NAME);

        /* Create local socket. */

        priv->listen_socket = socket(AF_UNIX, SOCK_STREAM, 0);
        if (priv->listen_socket == -1) {
            perror("socket");
            exit(EXIT_FAILURE);
        }

        /*
        * For portability clear the whole structure, since some
        * implementations have additional (nonstandard) fields in
        * the structure.
        */

        memset(&priv->addr, 0, sizeof(struct sockaddr_un));

        /* Bind socket to socket name. */

        priv->addr.sun_family = AF_UNIX;
        strncpy(priv->addr.sun_path, SOCKET_NAME, sizeof(priv->addr.sun_path) - 1);

        ret = bind(priv->listen_socket, (const struct sockaddr *) &priv->addr,
                sizeof(struct sockaddr_un));
        if (ret == -1) {
            perror("bind");
            exit(EXIT_FAILURE);
        }

        /*
        * Prepare for accepting connections. The backlog size is set
        * to 20. So while one request is being processed other requests
        * can be waiting.
        */

        ret = listen(priv->listen_socket, 20);
        if (ret == -1) {
            perror("listen");
            exit(EXIT_FAILURE);
        }
    }

    /* process generic options */
    xf86CollectInputOptions(pInfo, NULL);
    xf86ProcessCommonOptions(pInfo, pInfo->options);

    priv->valuators = valuator_mask_new(TOUCH_NUM_AXES);
    if (!priv->valuators)
        return BadValue;

    priv->slots = TOUCH_MAX_SLOTS;
    priv->abs_x_only = 1;
    priv->width = 1024;
    priv->height = 768;
    priv->pmax = 255;
    priv->thread = 0;

    fprintf(stderr, "xf86-input-neko: %s and S is %s\n", __FUNCTION__, s);

    /* Return the configured device */
    return Success;
}

_X_EXPORT InputDriverRec NEKO = {
    .driverVersion   = 1,
    .driverName      = "neko",
    .PreInit         = xf86NekoInit,
    .UnInit          = xf86NekoUninit,
    .module          = NULL,
    .default_options = NULL,
#ifdef XI86_DRV_CAP_SERVER_FD
    0                /* TODO add this capability */
#endif
};

static pointer xf86NekoPlug(pointer module,
            __attribute__ ((unused)) pointer options,
            __attribute__ ((unused)) int *errmaj,
            __attribute__ ((unused)) int *errmin)
{
    xf86AddInputDriver(&NEKO, module, 0);
    return module;
}

static XF86ModuleVersionInfo xf86NekoVersionRec = {
    "neko",
    MODULEVENDORSTRING,
    MODINFOSTRING1,
    MODINFOSTRING2,
    XORG_VERSION_CURRENT,
    PACKAGE_VERSION_MAJOR, PACKAGE_VERSION_MINOR, PACKAGE_VERSION_PATCHLEVEL,
    ABI_CLASS_XINPUT,
    ABI_XINPUT_VERSION,
    MOD_CLASS_XINPUT,
    {0, 0, 0, 0}    /* signature, to be patched into the file by a tool */
};

_X_EXPORT XF86ModuleData nekoModuleData = {
    .vers = &xf86NekoVersionRec,
    .setup = xf86NekoPlug,
    .teardown = NULL
};
