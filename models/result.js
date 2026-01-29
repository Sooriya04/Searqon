const mongoose = require("mongoose");

const resultSchema = new mongoose.Schema(
  {
    query: {
      type: String,
      required: true,
      index: true,
    },
    source: {
      type: String,
      required: true,
      index: true,
    },
    title: {
      type: String,
      required: true,
    },
    url: {
      type: String,
      required: true,
    },
    content: {
      type: String,
      required: true,
    },
    score: {
      type: Number,
      default: 0.5,
    },
    wordCount: {
      type: Number,
    },
    author: {
      type: String,
    },
    publishedDate: {
      type: String,
    },
    metadata: {
      type: mongoose.Schema.Types.Mixed,
    },
  },
  {
    timestamps: true,
  }
);

// Index for faster queries
resultSchema.index({ query: 1, source: 1 });
resultSchema.index({ createdAt: -1 });

const Result = mongoose.model("Result", resultSchema);

module.exports = Result;
