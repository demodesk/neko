#pragma once

#include <stdio.h>
#include <gst/gst.h>
#include <gst/app/gstappsrc.h>
#include <gst/video/video.h>

typedef struct GstPipelineCtx {
  int pipelineId;
  GstElement *pipeline;
  GstElement *appsink;
  GstElement *appsrc;
} GstPipelineCtx;

extern void goHandlePipelineBuffer(int pipelineId, void *buffer, int bufferLen, guint64 duration, gboolean isDeltaUnit);
extern void goPipelineLog(int pipelineId, char *level, char *msg);

GstPipelineCtx *gstreamer_pipeline_create(char *pipelineStr, int pipelineId, GError **error);
void gstreamer_pipeline_attach_appsink(GstPipelineCtx *ctx, char *sinkName);
void gstreamer_pipeline_attach_appsrc(GstPipelineCtx *ctx, char *srcName);
void gstreamer_pipeline_play(GstPipelineCtx *ctx);
void gstreamer_pipeline_pause(GstPipelineCtx *ctx);
void gstreamer_pipeline_destory(GstPipelineCtx *ctx);
void gstreamer_pipeline_push(GstPipelineCtx *ctx, void *buffer, int bufferLen);

gboolean gstreamer_pipeline_set_prop_int(GstPipelineCtx *ctx, char *binName, char *prop, gint value);
gboolean gstreamer_pipeline_set_caps_framerate(GstPipelineCtx *ctx, const gchar* binName, gint numerator, gint denominator);
gboolean gstreamer_pipeline_set_caps_resolution(GstPipelineCtx *ctx, const gchar* binName, gint width, gint height);
gboolean gstreamer_pipeline_emit_video_keyframe(GstPipelineCtx *ctx);
