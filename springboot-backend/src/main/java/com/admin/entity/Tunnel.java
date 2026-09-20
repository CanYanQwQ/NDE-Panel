package com.admin.entity;

import java.io.Serializable;
import java.math.BigDecimal;
import lombok.Data;
import lombok.EqualsAndHashCode;

/**
 * <p>
 * 隧道实体类
 * </p>
 *
 * @author QAQ
 * @since 2025-06-03
 */
@Data
@EqualsAndHashCode(callSuper = true)
public class Tunnel extends BaseEntity {

    private static final long serialVersionUID = 1L;

    /**
     * 隧道名称
     */
    private String name;

    /**
     * 入口节点ID
     */
    private Long inNodeId;

    /**
     * 入口IP (兼容字段)
     */
    private String inIp;

    /**
     * 出口节点ID
     */
    private Long outNodeId;

    /**
     * 出口IP (兼容字段)
     */
    private String outIp;

    /**
     * 隧道类型（1-端口转发，2-隧道转发）
     */
    private Integer type;

    /**
     * 流量计算类型（1 单向计算上传。2 双向）
     */
    private int flow;

    /**
     * 协议类型
     */
    private String protocol;

    /**
     * 流量倍率
     */
    private BigDecimal trafficRatio;


    private String tcpListenAddr;

    private String udpListenAddr;

    private String interfaceName;

    /**
     * 是否是搭协议时自动建的隧道(不入库,查询时现算)。
     *
     * 每台搭了协议的机器都会自动生成一条端口转发隧道,用来挂协议的 gost 转发。
     * 用户没手工建过却在隧道管理里看到它们,容易困惑,所以前端默认折叠隐藏。
     */
    @com.baomidou.mybatisplus.annotation.TableField(exist = false)
    private Boolean protocolManaged;
}
