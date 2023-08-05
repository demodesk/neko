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

#include <stdio.h>
#include <misc.h>
#include <xf86.h>
#if !defined(DGUX)
#include <xisb.h>
#endif
#include <xf86_OSproc.h>
#include <xf86Xinput.h>
#include <exevents.h>
#include <X11/keysym.h>
#include <mipointer.h>
#include <randrstr.h>
#include <xserver-properties.h>

#include <sys/time.h>
#include <time.h>
#include <stdint.h>
#include <fcntl.h>

#if defined (__FreeBSD__)
#include <dev/evdev/input.h>
#else
#include <linux/input.h>
#endif
#include<pthread.h>

#define TOUCH_MAX_SLOTS 10    /* fallback if not found */
#define TOUCH_SAMPLES_READ 3    /* up to, if available */
#define MAXBUTTONS 11        /* > 10 */
#define TOUCH_NUM_AXES 3    /* x, y, pressure */

struct tsdev;

struct neko_sample {
    int x;
    int y;
    unsigned int pressure;
    struct timeval tv;
};

struct neko_priv {
    pthread_t thread;
    struct tsdev *ts;
    int height;
    int width;
    int pmax;
    struct neko_sample last;
    ValuatorMask *valuators;
    int8_t abs_x_only;
    uint16_t slots;
    uint32_t *touchids;
};

static void xf86NekoReadInput(InputInfoPtr local)
{
    struct neko_priv *priv = (struct neko_priv *) (local->private);
    struct neko_sample samp;
    int ret;
    int type = 0;
    int i = 0;

    while (1) {
        // wait 1 second for data
        usleep(1000000);
        fprintf(stderr, "xf86-input-neko: read\n");

        samp.x = 100;
        samp.y = 100;

        if (priv->last.pressure == 0)
            samp.pressure = -1;
        else
            samp.pressure = 0;

        ValuatorMask *m = priv->valuators;
    
        if (priv->last.pressure == 0 && samp.pressure > 0) {
            type = XI_TouchBegin;
            fprintf(stderr, "xf86-input-neko: touch begin\n");
        } else if (priv->last.pressure > 0 && samp.pressure == 0) {
            type = XI_TouchEnd;
            fprintf(stderr, "xf86-input-neko: touch end\n");
        } else if (priv->last.pressure > 0 && samp.pressure > 0) {
            type = XI_TouchUpdate;
            fprintf(stderr, "xf86-input-neko: touch update\n");
        }
    
        valuator_mask_zero(m);
    
        if (type != XI_TouchEnd) {
            valuator_mask_set_double(m, 0, samp.x);
            valuator_mask_set_double(m, 1, samp.y);
            valuator_mask_set_double(m, 2, samp.pressure);
        }

        xf86PostTouchEvent(local->dev, i/2, type, 0, m);
    
        memcpy(&priv->last, &samp, sizeof(struct neko_sample));
        fprintf(stderr, "xf86-input-neko: read end, type is %d, i is %d\n", type, i/2);
        i++;
    }

    if (ret < 0) {
        xf86IDrvMsg(local, X_ERROR, "ts_read failed\n");
        return;
    }
}

static void xf86NekoInitButtonLabels(Atom *labels, size_t size)
{
    assert(size > 10);

    memset(labels, 0, size * sizeof(Atom));
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

#ifdef DEBUG
    xf86IDrvMsg(pInfo, X_ERROR, "%s\n", __FUNCTION__);
#endif

    switch (what) {
    case DEVICE_INIT:
        device->public.on = FALSE;

        memset(map, 0, sizeof(map));
        for (i = 0; i < MAXBUTTONS; i++)
            map[i + 1] = i + 1;

        xf86NekoInitButtonLabels(labels, ARRAY_SIZE(labels));

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
                           0,        /* min val */
                           priv->width - 1,    /* max val */
                           priv->width,    /* resolution */
                           0,        /* min_res */
                           priv->width,    /* max_res */
                           Absolute);

            InitValuatorAxisStruct(device, 1,
                           XIGetKnownProperty(AXIS_LABEL_PROP_ABS_Y),
                           0,        /* min val */
                           priv->height - 1,/* max val */
                           priv->height,    /* resolution */
                           0,        /* min_res */
                           priv->height,    /* max_res */
                           Absolute);

            InitValuatorAxisStruct(device, 2,
                           XIGetKnownProperty(AXIS_LABEL_PROP_ABS_PRESSURE),
                           0,        /* min val */
                           priv->pmax,    /* max val */
                           priv->pmax + 1,    /* resolution */
                           0,        /* min_res */
                           priv->pmax + 1,    /* max_res */
                           Absolute);
        } else {
            InitValuatorAxisStruct(device, 0,
                           XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_POSITION_X),
                           0,        /* min val */
                           priv->width - 1,    /* max val */
                           priv->width,    /* resolution */
                           0,        /* min_res */
                           priv->width,    /* max_res */
                           Absolute);

            InitValuatorAxisStruct(device, 1,
                           XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_POSITION_Y),
                           0,        /* min val */
                           priv->height - 1,/* max val */
                           priv->height,    /* resolution */
                           0,        /* min_res */
                           priv->height,    /* max_res */
                           Absolute);

            InitValuatorAxisStruct(device, 2,
                           XIGetKnownProperty(AXIS_LABEL_PROP_ABS_MT_PRESSURE),
                           0,        /* min val */
                           priv->pmax,    /* max val */
                           priv->pmax + 1,    /* resolution */
                           0,        /* min_res */
                           priv->pmax + 1,    /* max_res */
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
        fprintf(stderr, "xf86-input-neko: DEVICE ON\n");
        device->public.on = TRUE;

        if (priv->thread == 0)
            pthread_create(&priv->thread, NULL, (void *)xf86NekoReadInput, pInfo);
        break;

    case DEVICE_OFF:
    case DEVICE_CLOSE:
        fprintf(stderr, "xf86-input-neko: DEVICE OFF\n");
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

#ifdef DEBUG
    xf86IDrvMsg(pInfo, X_ERROR, "%s\n", __FUNCTION__);
#endif

    if (priv->thread) {
        pthread_cancel(priv->thread);
        pthread_join(priv->thread, NULL);
        priv->thread = 0;
    }

    valuato_rmask_free(&priv->valuators);
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
